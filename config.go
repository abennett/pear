package main

import (
	"errors"
	"os"
	"reflect"
	"strings"
)

var ()

type Config struct {
	SlackSecret string `env:"SLACK_SECRET,required"`
	SlackToken  string `env:"SLACK_TOKEN,required"`
	Channel     string `env:"SLACK_CHANNEL,required"`
	DatabaseUrl string `env:"DATABASE_URL,required"`
	Port        string `env:"PORT,required"`
	Debug       bool   `env:"DEBUG"`
}

func NewConfig() (*Config, error) {
	var conf Config
	val := reflect.ValueOf(&conf)
	typ := reflect.TypeOf(conf)
	for x := 0; x < typ.NumField(); x++ {
		tag := typ.Field(x).Tag
		v, ok := tag.Lookup("env")
		if !ok {
			continue
		}
		v, err := processTag(v)
		if err != nil {
			return nil, err
		}
		switch kind := typ.Field(x).Type.Kind(); kind {
		case reflect.String:
			val.Elem().Field(x).SetString(v)
		case reflect.Bool:
			val.Elem().Field(x).SetBool(ok)
		default:
			return nil, errors.New("unknown config type " + kind.String())
		}
	}
	return &conf, nil
}

func processTag(tag string) (string, error) {
	split := strings.Split(tag, ",")
	var required bool
	if len(split) > 1 {
		required = true
	}
	env, ok := os.LookupEnv(split[0])
	if !ok && required {
		return "", errors.New("missing envvar: " + split[0])
	}
	return env, nil
}
