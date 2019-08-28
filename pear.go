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

type Pear struct {
	ID     int       `db:"id"`
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
	var msg *slack.Msg
	var err error
	switch action.ActionID {
	case SeedSubmit:
		ps.logger.Debug("handling seed submitSeed")
		m, seed, err := ps.submitSeed(ic)
		if err != nil {
			return err
		}
		msg = m
		if err = ps.growSeed(seed); err != nil {
			ps.logger.Error("error posting slack message", "error", err)
			return err
		}
	case SeedCancel:
		ps.logger.Debug("handling cancelSeed")
		// do cancel stuff
		msg = &slack.Msg{DeleteOriginal: true}
	case PickPear:
		// do pear stuff
	}
	if msg == nil {
		return errors.New("msg is nil")
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	resp, err := http.Post(ic.ResponseURL, ApplicationJson, bytes.NewReader(b))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(b))
	}
	return nil
}

func (ps *PearService) growSeed(seed *Seed) error {
	block := formatGrowMsg(seed)
	b, _ := json.Marshal(&block)
	ps.logger.Debug("outgoing message", "msg", string(b))
	_, _, err := ps.client.PostMessage(ps.channel, slack.MsgOptionBlocks(block))
	return err
}

func formatGrowMsg(seed *Seed) slack.Block {
	pickBtnText := slack.NewTextBlockObject(slack.PlainTextType, "Pick :pear:", false, false)
	pickBtn := slack.NewButtonBlockElement(SeedCancel, strconv.Itoa(seed.ID), pickBtnText)
	btnAccessory := slack.NewAccessory(pickBtn)

	req := fmt.Sprintf("<@%s> wants to learn %s. :seedling:", seed.Sower, seed.Topic)
	headerText := slack.NewTextBlockObject(slack.MarkdownType, req, false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, btnAccessory)

	return headerSection
}

func (ps *PearService) submitSeed(ic *slack.InteractionCallback) (*slack.Msg, *Seed, error) {
	seed := &Seed{
		Sower:   ic.User.ID,
		Planted: time.Now(),
		Topic:   ic.ActionCallback.BlockActions[0].Value,
	}
	id, err := ps.PlantSeed(seed)
	seed.ID = id
	if err != nil {
		return nil, seed, err
	}
	return seedPlantedMsg(), seed, nil
}

func seedPlantedMsg() *slack.Msg {
	headerText := slack.NewTextBlockObject(slack.MarkdownType, "Pear seed planted! :seedling:", false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)
	blocks := CombineBlocks(headerSection)
	return &slack.Msg{
		ResponseType:    "ephemeral",
		ReplaceOriginal: true,
		Blocks:          blocks,
	}
}

func (ps *PearService) PlantSeed(s *Seed) (int, error) {
	rows, err := ps.db.NamedQuery("INSERT INTO seed (sower, topic, planted) VALUES (:sower, :topic, now()) RETURNING id", s)
	defer rows.Close()
	var id int
	rows.Next()
	err = rows.Scan(&id)
	if err != nil {
		return -1, err
	}
	return id, nil
}

func SlashResponse(topic string) slack.Msg {
	resp := fmt.Sprintf("Do you want to submit a request to: *%s*", topic)
	headerText := slack.NewTextBlockObject(slack.MarkdownType, resp, false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	submitBtnText := slack.NewTextBlockObject(slack.PlainTextType, "Submit", false, false)
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

type Seed struct {
	ID      int       `db:"id"`
	Sower   string    `db:"sower"`
	Topic   string    `db:"topic"`
	Planted time.Time `db:"planted"`
}

func HarvestSeed(sc slack.SlashCommand) *Seed {
	return &Seed{
		Sower:   sc.UserID,
		Topic:   sc.Text,
		Planted: time.Now(),
	}
}
