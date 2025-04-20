package relayer

import (
	"context"
	"errors"

	"github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
)

// SendGenesisTransfer sends a genesis transfer from a rollapp to a hub chain
func SendGenesisTransfer(
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
	res, err := ra.TrySendGenesisTransfer(ctx, channel.ChannelId)
	if err != nil {
		return err
	}

	// For rollapp chain, we use height+1 for proof height
	srch := uint64(res.Height + 1)

	// FIXME: wait  for state committed

	dsth, err := hubC.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return err
	}

	var srcMsgs, dstMsgs []provider.RelayerMessage
	// Use sequence 0 for genesis transfer
	// FIXME: get correct sequence (from query or from tx's events)
	seq := uint64(0)

	err = AddMessagesForSequences(
		ctx,
		[]uint64{seq},
		raC, hubC,
		int64(srch), int64(dsth),
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

	if err := msgs.PrependMsgUpdateClient(ctx, raC, hubC, int64(srch), dsth); err != nil {
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
