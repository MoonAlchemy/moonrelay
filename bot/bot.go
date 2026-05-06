package bot

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	tele "gopkg.in/telebot.v4"
)

func StartBot() {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("You did not provide a TELEGRAM_TOKEN")
	}

	log.Println("Starting bot...")

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)

	}
	b.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			allowedID, _ := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
			if c.Sender().ID != allowedID {
				return c.Send("Unauthorized")
			}
			return next(c)
		}
	})
	b.Handle("/start", func(c tele.Context) error {
		return c.Send("bot is alive")
	})
	b.Handle("/stop_stream", func(c tele.Context) error {
		err := StopStream()
		if err != nil {
			return c.Send("Failed to stop stream: " + err.Error())
		}
		return c.Send("Stream stopped")
	})
	b.Handle("/status", func(c tele.Context) error {
		status, err := Status()
		if err != nil {
			return c.Send("Error checking status: " + err.Error())
		}
		if status == "" {
			return c.Send("Stream is not running")
		}
		return c.Send("Stream is running: " + status)
	})

	log.Println("Bot started successfully")
	b.Start()
}

func StopStream() error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer cli.Close()
	return cli.ContainerStop(context.Background(), "moonrelay", container.StopOptions{})
}

func Status() (string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return "", err
	}
	defer cli.Close()

	info, err := cli.ContainerInspect(context.Background(), "moonrelay")
	if err != nil {
		return "", err
	}
	return info.State.Status, nil
}
