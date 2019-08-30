package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/jmoiron/sqlx"
	"github.com/nlopes/slack"
)

const (
	AppName = "pear"

	SeedBlock  = "seed-block"
	SeedSubmit = "submit-seed"
	SeedCancel = "cancel-seed"

	PickBlock = "pear-block"
	PickPear  = "pick-pear"
)

var (
	ErrInvalidToken = errors.New("slack token is invalid")
)

type PearService struct {
	client  *slack.Client
	db      *sqlx.DB
	secret  string
	channel string
	logger  hclog.Logger
}

type Seed struct {
	ID      int       `db:"id"`
	Sower   string    `db:"sower"`
	Topic   string    `db:"topic"`
	Planted time.Time `db:"planted"`
}

type Pear struct {
	ID     int       `db:"id"`
	SeedID int       `db:"seed_id"`
	Picker string    `db:"picker"`
	Picked time.Time `db:"picked"`
}

func NewPearService(conf *Config, logger hclog.Logger) *PearService {
	db, err := InitPG(conf.DatabaseUrl)
	if err != nil {
		panic(err)
	}
	return &PearService{
		client:  slack.New(conf.SlackToken),
		secret:  conf.SlackSecret,
		db:      db,
		channel: conf.Channel,
		logger:  logger,
	}
}

func (ps *PearService) VerifyRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verifier, err := slack.NewSecretsVerifier(r.Header, ps.secret)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		verifier.Write(b)
		if err = verifier.Ensure(); err != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		r.Body = ioutil.NopCloser(bytes.NewReader(b))
		next.ServeHTTP(w, r)
	})
}

func (ps *PearService) HandleNew(sc slack.SlashCommand) (slack.Msg, error) {
	return SlashResponse(sc.Text), nil
}

func (ps *PearService) HandleSubmit(ic *slack.InteractionCallback) error {
	if len(ic.ActionCallback.BlockActions) != 1 {
		return errors.New("invalid number of block actions")
	}

	action := ic.ActionCallback.BlockActions[0]
	var err error
	switch action.ActionID {
	case SeedSubmit:
		ps.logger.Debug("handling seed submitSeed")
		seed, err := ps.submitSeed(ic)
		if err != nil {
			return err
		}
		if err = ps.growSeed(seed); err != nil {
			ps.logger.Error("error posting slack message", "error", err)
			return err
		}
	case SeedCancel:
		// do cancel stuff
		ps.logger.Debug("canceling seed")
		err = ps.postMsg(ic, slack.Msg{DeleteOriginal: true})
	case PickPear:
		ps.logger.Debug("picking pear")
		err := ps.PickPear(ic)
		if err != nil {
			return err
		}
	default:
		ps.logger.Debug("using default")
		err = ps.postMsg(ic, slack.Msg{DeleteOriginal: true})
	}
	return err
}

func (ps *PearService) postMsg(ic *slack.InteractionCallback, msgs ...slack.Msg) error {
	for _, msg := range msgs {
		b, _ := json.Marshal(&msg)
		ps.logger.Debug("outgoing message", "msg", string(b))
		resp, err := http.Post(ic.ResponseURL, ApplicationJson, bytes.NewReader(b))
		if err != nil {
			ps.logger.Error("error responding", "status_code", resp.StatusCode)
			return err
		}
		if ps.logger.IsDebug() {
			b, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			ps.logger.Debug("response", "resp", string(b))
		}
	}
	return nil
}

func (ps *PearService) growSeed(seed *Seed) error {
	block := formatGrowMsg(seed)
	_, _, err := ps.client.PostMessage(ps.channel, slack.MsgOptionBlocks(block))
	return err
}

func formatGrowMsg(seed *Seed) slack.Block {
	pickBtnText := slack.NewTextBlockObject(slack.PlainTextType, "Pick :pear:", false, false)
	pickBtn := slack.NewButtonBlockElement(PickPear, strconv.Itoa(seed.ID), pickBtnText)
	btnAccessory := slack.NewAccessory(pickBtn)

	req := fmt.Sprintf("<@%s> wants to learn %s. :seedling:", seed.Sower, seed.Topic)
	headerText := slack.NewTextBlockObject(slack.MarkdownType, req, false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, btnAccessory)

	return headerSection
}

func (ps *PearService) submitSeed(ic *slack.InteractionCallback) (*Seed, error) {
	seed := &Seed{
		Sower:   ic.User.ID,
		Planted: time.Now(),
		Topic:   ic.ActionCallback.BlockActions[0].Value,
	}
	if seed.Topic == "" {
		return nil, errors.New("topic required")
	}
	id, err := ps.PlantSeed(seed)
	seed.ID = id
	if err != nil {
		return seed, err
	}
	msg := seedPlantedMsg()
	if err = ps.postMsg(ic, msg); err != nil {
		return seed, err
	}
	return seed, nil
}

func seedPlantedMsg() slack.Msg {
	headerText := slack.NewTextBlockObject(slack.MarkdownType, "Pear seed planted! :seedling:", false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)
	blocks := CombineBlocks(headerSection)
	return slack.Msg{
		ResponseType:    "ephemeral",
		ReplaceOriginal: true,
		Blocks:          blocks,
	}
}

func (ps *PearService) FetchSeed(id int) (*Seed, error) {
	var seed Seed
	err := ps.db.Get(&seed, "SELECT * from seed WHERE id = $1", id)
	if err != nil {
		return nil, fmt.Errorf("error fetching seed: %v", err)
	}
	return &seed, nil
}

func (ps *PearService) PlantSeed(s *Seed) (int, error) {
	rows, err := ps.db.NamedQuery("INSERT INTO seed (sower, topic, planted) VALUES (:sower, :topic, now()) RETURNING id", s)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	var id int
	rows.Next()
	err = rows.Scan(&id)
	if err != nil {
		return -1, err
	}
	return id, nil
}

func (ps *PearService) PickPear(ic *slack.InteractionCallback) error {
	seedID, err := strconv.Atoi(ic.ActionCallback.BlockActions[0].Value)
	if err != nil {
		return err
	}
	pear := &Pear{
		SeedID: seedID,
		Picker: ic.User.ID,
	}
	_, err = ps.StorePear(pear)
	if err != nil {
		return err
	}
	msg, err := formatPickResponse(ic)
	if err != nil {
		return err
	}
	if err = ps.postMsg(ic, msg); err != nil {
		return fmt.Errorf("unable to post pick response: %v", err)
	}
	seed, err := ps.FetchSeed(seedID)
	if err != nil {
		return err
	}
	channel, _, _, err := ps.client.OpenConversation(
		&slack.OpenConversationParameters{
			Users: []string{ic.User.ID, seed.Sower},
		},
	)
	if err != nil {
		return err
	}
	pmMsg := fmt.Sprintf(
		"<@%s> offered to help you learn %s!\nBear Fruit! :pear:",
		ic.User.ID,
		seed.Topic,
	)
	_, _, err = ps.client.PostMessage(channel.ID, slack.MsgOptionText(pmMsg, false))
	return err
}

func formatPickResponse(ic *slack.InteractionCallback) (slack.Msg, error) {
	resp := fmt.Sprintf("<@%s> picked this pear.", ic.User.ID)
	contextText := slack.NewTextBlockObject(slack.MarkdownType, resp, false, false)
	contextBlock := slack.NewContextBlock("", contextText)

	var msgText *slack.TextBlockObject
	for _, block := range ic.Message.Blocks.BlockSet {
		if section, ok := block.(*slack.SectionBlock); ok {
			msgText = section.Text
			break
		}
		return slack.Msg{}, errors.New("unable to find sectionBlock")
	}

	sectionBlock := slack.NewSectionBlock(msgText, nil, nil)
	blocks := CombineBlocks(sectionBlock, contextBlock)
	return slack.Msg{
		ReplaceOriginal: true,
		Blocks:          blocks,
	}, nil
}

func (ps *PearService) StorePear(pear *Pear) (int, error) {
	rows, err := ps.db.NamedQuery("INSERT INTO pear (seed_id, picker, picked) VALUES (:seed_id, :picker, now()) RETURNING id", pear)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	var id int
	err = rows.Scan(&id)
	if err != nil {
		return -1, nil
	}
	return id, nil
}

func SlashResponse(topic string) slack.Msg {
	resp := fmt.Sprintf("Do you want to learn *%s*?", topic)
	headerText := slack.NewTextBlockObject(slack.MarkdownType, resp, false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	submitBtnText := slack.NewTextBlockObject(slack.PlainTextType, "Yes", false, false)
	submitBtn := slack.NewButtonBlockElement(SeedSubmit, topic, submitBtnText)

	cancelBtnText := slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false)
	cancelBtn := slack.NewButtonBlockElement(SeedCancel, "cancel", cancelBtnText)

	actionBlock := slack.NewActionBlock(SeedBlock, submitBtn, cancelBtn)

	blocks := CombineBlocks(headerSection, actionBlock)

	return slack.Msg{
		ResponseType: "ephemeral",
		Blocks:       blocks,
	}
}

func CombineBlocks(blocks ...slack.Block) slack.Blocks {
	return slack.Blocks{
		BlockSet: blocks,
	}
}

func HarvestSeed(sc slack.SlashCommand) *Seed {
	return &Seed{
		Sower:   sc.UserID,
		Topic:   sc.Text,
		Planted: time.Now(),
	}
}
