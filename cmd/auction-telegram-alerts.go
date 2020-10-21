package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/rpc/client"
)

type BlockEvents struct {
	MintBondedRatio             []string `json:"mint.bonded_ratio"`
	ProposerRewardAmount        []string `json:"proposer_reward.amount"`
	CommissionValidator         []string `json:"commission.validator"`
	KavadistKavaDistInflation   []string `json:"kavadist.kava_dist_inflation"`
	SwapsExpiredAtomicSwapIds   []string `json:"swaps_expired.atomic_swap_ids"`
	SwapsExpiredExpirationBlock []string `json:"swaps_expired.expiration_block"`
	RewardsValidator            []string `json:"rewards.validator"`
	LivenessMissedBlocks        []string `json:"liveness.missed_blocks"`
	ProposerRewardValidator     []string `json:"proposer_reward.validator"`
	TmEvent                     []string `json:"tm.event"`
	TransferRecipient           []string `json:"transfer.recipient"`
	MintInflation               []string `json:"mint.inflation"`
	LivenessHeight              []string `json:"liveness.height"`
	LivenessAddress             []string `json:"liveness.address"`
	TransferAmount              []string `json:"transfer.amount"`
	MessageSender               []string `json:"message.sender"`
	RewardsAmount               []string `json:"rewards.amount"`
	MintAnnualProvisions        []string `json:"mint.annual_provisions"`
	CommissionAmount            []string `json:"commission.amount"`
	MintAmount                  []string `json:"mint.amount"`
	AuctionStartAuctionID       []string `json:"auction_start.auction_id"`
	AuctionStartAuctionType     []string `json:"auction_start.auction_type"`
	AuctionStartBid             []string `json:"auction_start.bid"`
	AuctionStartLot             []string `json:"auction_start.lot"`
	AuctionStartMaxBid          []string `json:"auction_start.max_bid"`
}

type AuctionAlert struct {
	ID          string `json:"id" yaml:"id"`
	AuctionType string `json:"auction_type" yaml:"auction_type"`
	Bid         string `json:"bid" yaml:"bid"`
	Lot         string `json:"lot" yaml:"lot"`
	MaxBid      string `json:"max_bid" yaml:"max_bid"`
}

func NewAuctionAlert(id, atype, bid, lot, maxbid string) AuctionAlert {
	return AuctionAlert{
		ID:          id,
		AuctionType: atype,
		Bid:         bid,
		Lot:         lot,
		MaxBid:      maxbid,
	}
}

func (aa AuctionAlert) String() string {
	return fmt.Sprintf(`
	New Auction Started:
	ID: %s
	Type: %s
	Bid: %s
	Lot %s
	Max Bid %s
	`, aa.ID, aa.AuctionType, aa.Bid, aa.Lot, aa.MaxBid)
}

func SendTelegramMessage(botId string, chatId string, msg string) {
	if botId == "" || chatId == "" || msg == "" {
		return
	}

	// Prepend deputy ID to msg
	tgMsg := fmt.Sprintf("%s", msg)

	// timeout requests to avoid unexpected delays from to logging
	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}
	endPoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botId)
	formData := url.Values{
		"chat_id":    {chatId},
		"parse_mode": {"html"},
		"text":       {tgMsg},
	}
	log.Printf("send tg message, bot_id=%s, chat_id=%s, msg=%s", botId, chatId, tgMsg)
	res, err := httpClient.PostForm(endPoint, formData)
	if err != nil {
		log.Printf("send telegram message error, bot_id=%s, chat_id=%s, msg=%s, err=%s", botId, chatId, tgMsg, err.Error())
		return
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		log.Printf("read http response error, err=%s", err.Error())
		return
	}
	log.Printf("tg response: %s", string(bodyBytes))
}

func SubscribeAuctionsCmd(cdc *codec.Codec) *cobra.Command {
	var nodeAddress string
	var telegramBotID string
	var telegarmChatID string

	cmd := &cobra.Command{
		Use:   "subscribe-auctions",
		Short: "Listen for auction events on a node and print them out.",
		Long:  `Subscribe to auction events produced by a node.`,
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {

			c, err := client.NewHTTP(nodeAddress, "/websocket")
			if err != nil {
				return fmt.Errorf("can't connect to node: %w", err)
			}
			err = c.Start() // just call this undocumented function otherwise c.Subscribe panics with a cryptic error
			if err != nil {
				return fmt.Errorf("can't connect to node: %w", err)
			}

			ch, err := c.Subscribe(context.Background(), "subscriber", "tm.event='NewBlock'")
			if err != nil {
				return fmt.Errorf("can't subscribe to node: %w", err)
			}

			fmt.Println("listening...")
			for {
				event := <-ch

				bz, err := cdc.MarshalJSONIndent(event.Events, "", "  ")
				if err != nil {
					panic(err)
				}
				var blockEvents BlockEvents
				err = cdc.UnmarshalJSON(bz, &blockEvents)
				if err != nil {
					log.Fatalf("failed to unmarshal from events: %s ", err)
				}
				if len(blockEvents.AuctionStartAuctionID) > 0 {
					for idx := range blockEvents.AuctionStartAuctionID {
						alert := NewAuctionAlert(blockEvents.AuctionStartAuctionID[idx], blockEvents.AuctionStartAuctionType[idx], blockEvents.AuctionStartBid[idx], blockEvents.AuctionStartLot[idx], blockEvents.AuctionStartMaxBid[idx])
						log.Println(alert)
						SendTelegramMessage(telegramBotID, telegarmChatID, alert.String())
					}
				}

				// TODO graceful shutdown
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&nodeAddress, "node", "http://localhost:26657", "rpc node address")
	cmd.Flags().StringVar(&telegramBotID, "bot-id", "", "telegram bot id")
	cmd.Flags().StringVar(&telegarmChatID, "chat-id", "", "telegram chat id")
	return cmd
}
