package relayer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// SendAndRelayGenesisTransfer sends a genesis transfer from a rollapp to a hub chain
func SendAndRelayGenesisTransfer(
	ctx context.Context,
	hubC *Chain,
	raC *Chain,
) error {
	// get active channel
	channel, err := getActiveChannelForGenesisBridge(ctx, raC)
	if err != nil {
		return err
	}

	ra, ok := raC.ChainProvider.(provider.RollappProvider)
	if !ok {
		return errors.New("not rollapp provider")
	}
	hub, ok := hubC.ChainProvider.(provider.DymensionHubProvider)
	if !ok {
		return errors.New("not dymension hub provider")
	}

	hubH, raH, err := QueryLatestHeights(ctx, hubC, raC)
	if err != nil {
		return fmt.Errorf("querying latest heights: %w", err)
	}

	// For rollapp chain, we use proofH = transfer_height+1 for proof height
	var proofH, seq uint64
	commitments, err := raC.ChainProvider.QueryPacketCommitments(ctx, uint64(raH), channel.ChannelId, channel.PortId)
	if err != nil || len(commitments.Commitments) > 1 {
		return fmt.Errorf("query packet commitments: %w", err)
	}

	// Unanswered transfer can happen if genesis bridge already committed but not relayed
	if len(commitments.Commitments) == 1 {
		seq = commitments.Commitments[0].Sequence
		msgTransfer, err := raC.ChainProvider.QuerySendPacket(ctx, channel.ChannelId, channel.PortId, seq)
		if err != nil {
			return err
		}
		raC.log.Info("genesis packet is in-flight.", zap.Any("msgTransfer", msgTransfer))
		// proofH = msgTransfer.Height + 1 // doesn't work
		proofH = uint64(raH)
	} else {
		res, err := ra.TrySendGenesisTransfer(ctx, channel.ChannelId)
		if err != nil {
			return err
		}
		proofH = uint64(res.Height + 1)

		commitments, err := raC.ChainProvider.QueryPacketCommitments(ctx, uint64(res.Height), channel.ChannelId, channel.PortId)
		if err != nil || len(commitments.Commitments) != 1 {
			return fmt.Errorf("query packet commitments: %w", err)
		}
		seq = commitments.Commitments[0].Sequence
	}

	// wait for state committed
	hubC.log.Info("Waiting for state committed", zap.Int64("height", int64(proofH)), zap.String("chain_id", raC.ChainID()))
	err = retry.Do(func() error {
		committedH, err := hub.GetLatestRollappStateHeight(ctx, raC.ChainID())
		if err != nil {
			return fmt.Errorf("get latest rollapp state height: %w", err)
		}
		if committedH < int64(proofH) {
			hubC.log.Info("Waiting for state committed", zap.Int64("height", int64(proofH)), zap.Int64("committed_height", committedH))
			return fmt.Errorf("rollapp state not yet committed at height %d (current: %d)", proofH, committedH)
		}
		return nil
	},
		retry.Attempts(0), // forever
		retry.Delay(2*time.Second),
		retry.MaxDelay(10*time.Second),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			hubC.log.Info("Retrying to check rollapp state commitment", zap.Uint("attempt", n), zap.Error(err))
		}),
	)
	if err != nil {
		return err
	}

	var srcMsgs, dstMsgs []provider.RelayerMessage
	err = AddMessagesForSequences(
		ctx,
		[]uint64{seq},
		raC, hubC,
		int64(proofH), int64(hubH),
		&srcMsgs, &dstMsgs,
		channel.ChannelId, channel.PortId,
		channel.Counterparty.ChannelId, channel.Counterparty.PortId,
		channel.Ordering,
	)
	if err != nil {
		raC.log.Error(
			"Failed to construct messages for genesis transfer",
			zap.String("src_chain_id", raC.ChainID()),
			zap.String("src_channel_id", channel.ChannelId),
			zap.String("src_port_id", channel.PortId),
			zap.String("dst_chain_id", hubC.ChainID()),
			zap.String("dst_channel_id", channel.Counterparty.ChannelId),
			zap.String("dst_port_id", channel.Counterparty.PortId),
			zap.String("channel_order", channel.Ordering.String()),
			zap.Error(err),
		)
		return err
	}

	// Set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Dst:          dstMsgs,
		MaxTxSize:    TwoMB,
		MaxMsgLength: DefaultMaxMsgLength,
	}

	if !msgs.Ready() {
		raC.log.Info(
			"No packets to relay.",
			zap.String("src_chain_id", raC.ChainID()),
			zap.String("src_port_id", channel.PortId),
			zap.String("dst_chain_id", hubC.ChainID()),
			zap.String("dst_port_id", channel.Counterparty.PortId),
		)
		return nil
	}

	if err := msgs.PrependMsgUpdateClient(ctx, raC, hubC, int64(proofH), hubH); err != nil {
		return err
	}

	// Send messages to their respective chains
	result := msgs.Send(ctx, raC.log, AsRelayMsgSender(raC), AsRelayMsgSender(hubC), "")
	if err := result.Error(); err != nil {
		if result.PartiallySent() {
			raC.log.Info(
				"Partial success when relaying packets.",
				zap.String("src_chain_id", raC.ChainID()),
				zap.String("src_port_id", channel.PortId),
				zap.String("dst_chain_id", hubC.ChainID()),
				zap.String("dst_port_id", channel.Counterparty.PortId),
				zap.Error(err),
			)
		}
		return err
	}

	if result.SuccessfulSrcBatches > 0 {
		raC.logPacketsRelayed(hubC, result.SuccessfulSrcBatches, channel)
	}
	if result.SuccessfulDstBatches > 0 {
		hubC.logPacketsRelayed(raC, result.SuccessfulDstBatches, channel)
	}

	return nil
}

// Adhering to the dymension canonical light client protocol,
// 1. we wait until the rollapp state of the light client height is committed
// 2. we succesfully set the light client as canonical for the rollapp
// Assumes c is the Hub.
// Blocks the thread
func BlockUntilClientIsCanonical(ctx context.Context, c *Chain, rollappID string, dsth int64) error {
	hub, ok := c.ChainProvider.(provider.DymensionHubProvider)
	if !ok {
		return errors.New("not dymension hub provider")
	}

	// check if rollapp already has a canonical client
	canonicalClient, err := hub.GetCanonicalClient(ctx, rollappID)
	if err == nil {
		c.log.Info("Rollapp already has a canonical client", zap.String("chain_id", rollappID), zap.String("client_id", canonicalClient))
		return nil
	}

	// wait for state committed
	c.log.Info("Waiting for state committed", zap.Int64("height", int64(dsth)), zap.String("chain_id", rollappID))
	err = retry.Do(func() error {
		committedH, err := hub.GetLatestRollappStateHeight(ctx, rollappID)
		if err != nil {
			return fmt.Errorf("get latest rollapp state height: %w", err)
		}
		if committedH < int64(dsth) {
			c.log.Info("Waiting for state committed", zap.Int64("height", int64(dsth)), zap.Int64("committed_height", committedH))
			return fmt.Errorf("rollapp state not yet committed at height %d (current: %d)", dsth, committedH)
		}
		return nil
	},
		retry.Attempts(0), // forever
		retry.Delay(20*time.Second),
		retry.MaxDelay(time.Minute),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			c.log.Info("Retrying to check rollapp state commitment", zap.Uint("attempt", n), zap.Error(err))
		}),
	)
	if err != nil {
		return err
	}

	err = hub.TrySetCanonicalClient(ctx, c.PathEnd.ClientID)
	if err != nil {
		return fmt.Errorf("set canonical client: %w", err)
	}

	return nil
}

func getActiveChannelForGenesisBridge(ctx context.Context, src *Chain) (*chantypes.IdentifiedChannel, error) {
	// Query the list of channels on the src connection.
	srcChannels, err := queryChannelsOnConnection(ctx, src)
	if err != nil {
		return nil, err
	}

	// filter out only the channels in the OPEN state.
	var activeChannel *chantypes.IdentifiedChannel
	for _, channel := range srcChannels {
		if channel.State == types.OPEN {
			activeChannel = channel
		}
	}

	return activeChannel, nil
}
