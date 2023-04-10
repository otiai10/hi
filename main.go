package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/otiai10/openaigo"
)

func init() {
	flag.Parse()
}

func main() {
	ctx := context.Background()
	conversation := []openaigo.ChatMessage{
		{Role: "user", Content: strings.Join(os.Args, " ")},
	}
	chat(ctx, conversation)
}

func chat(ctx context.Context, conv []openaigo.ChatMessage) ([]openaigo.ChatMessage, error) {
	client := openaigo.NewClient(os.Getenv("OPENAI_API_KEY"))

	msg := make(chan openaigo.ChatCompletionResponse)
	dch := make(chan error)
	callback := func(res openaigo.ChatCompletionResponse, done bool, err error) {
		if done || err != nil {
			dch <- err
			return
		}
		msg <- res
	}

	if _, err := client.Chat(ctx, openaigo.ChatCompletionRequestBody{
		Model:          openaigo.GPT3_5Turbo,
		StreamCallback: callback,
		Messages:       conv,
	}); err != nil {
		log.Println(err)
		return cleanup(err, msg, dch)
	}

	answer, err := absorb(msg, dch)
	if err != nil {
		return cleanup(err, msg, dch)
	}
	conv = append(conv, openaigo.ChatMessage{
		Role: "assistant", Content: answer,
	})

	switch {
	case strings.HasSuffix(answer, "?"):
	default:
		fmt.Print("\nYou> ")
		r := bufio.NewReader(os.Stdin)
		s, err := r.ReadString('\n')
		if err != nil {
			return cleanup(err, msg, dch)
		}
		if shouldEndConversationFromUserSide(s) {
			return cleanup(err, msg, dch)
		}
		conv = append(conv, openaigo.ChatMessage{
			Role: "user", Content: s,
		})
		cleanup(nil, msg, dch)
		return chat(ctx, conv)
	}
	return conv, err
}

func shouldEndConversationFromUserSide(s string) bool {
	switch strings.Trim(s, "\n") {
	case "bye", "Bye", "end", "End", "quit", "Quit", "q":
		return true
	default:
		return false
	}
}

func absorb(msg chan openaigo.ChatCompletionResponse, dch chan error) (string, error) {
	var answer string
	var err error
	for {
		select {
		case r := <-msg:
			fmt.Print(r.Choices[0].Delta.Content)
			answer += r.Choices[0].Delta.Content
		case err = <-dch:
			if err != nil {
				log.Println(err)
			}
			return answer, err
		}
	}
}

func cleanup(err error, msg chan openaigo.ChatCompletionResponse, dch chan error) ([]openaigo.ChatMessage, error) {
	var empty []openaigo.ChatMessage
	close(msg)
	close(dch)
	return empty, err
}
