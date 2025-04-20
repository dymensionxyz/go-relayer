package relayer

import (
	"context"
	"errors"

	"github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	chantypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/rs/zerolog/log"
	"go.uber.org/zap"
)

func SendGenesisTransfer(
	ctx context.Context,
	hubC *Chain,
	raC *Chain,
) error {
	hub, ok := hubC.ChainProvider.(provider.DymensionHubProvider)
	if !ok {
		return errors.New("not dymension hub provider")
	}

	ra, ok := raC.ChainProvider.(provider.RollappProvider)
	if !ok {
		return errors.New("not rollapp provider")
	}

	// get active channel
	channel, err := getActiveChannelForGenesisBridge(ctx, raC)
	if err != nil {
		return err
	}

	// func (pp *PathProcessor) IsRelevantChannel(chainID string, channelID string) bool {

	res, err := ra.TrySendGenesisTransfer(ctx, channel.ChannelId)
	if err != nil {
		return err
	}
	// now we have txhash and height

	// Fetch any unrelayed sequences depending on the channel order
	// sp := relayer.UnrelayedSequences(ctx, hubC., dst, srcChannel)
	// FIXME: get seq number (either from the events, or using some query)
	seq := uint64(0)

	// for rollapp chain, we use height+1 for proof height
	srch := uint64(res.Height + 1)

	// FIXME: wait until state committed

	dsth, err := hubC.ChainProvider.QueryLatestHeight(ctx)
	if err != nil {
		return err
	}

	var msgsSrc []provider.RelayerMessage
	recvMsg, _, err := raC.ChainProvider.RelayPacketFromSequence(
		ctx,
		raC.ChainProvider,
		uint64(srch), uint64(dsth),
		seq,
		channel.ChannelId, channel.PortId,
		channel.Ordering,
	)
	if err != nil || recvMsg == nil {
		raC.log.Error(
			"Failed to relay genesis transfer",
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

	// set the maximum relay transaction constraints
	msgs := &RelayMsgs{
		Src:          append(msgsSrc),
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

	if err := msgs.PrependMsgUpdateClient(ctx, src, dst, srch, dsth); err != nil {
		return err
	}

	// send messages to their respective chains
	result := msgs.Send(ctx, log, AsRelayMsgSender(src), AsRelayMsgSender(dst), memo)
	if err := result.Error(); err != nil {
		if result.PartiallySent() {
			log.Info(
				"Partial success when relaying packets.",
				zap.String("src_chain_id", src.ChainID()),
				zap.String("src_port_id", srcChannel.PortId),
				zap.String("dst_chain_id", dst.ChainID()),
				zap.String("dst_port_id", srcChannel.Counterparty.PortId),
				zap.Error(err),
			)
		}
		return err
	}

	if result.SuccessfulSrcBatches > 0 {
		src.logPacketsRelayed(dst, result.SuccessfulSrcBatches, srcChannel)
	}
	if result.SuccessfulDstBatches > 0 {
		dst.logPacketsRelayed(src, result.SuccessfulDstBatches, srcChannel)
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
