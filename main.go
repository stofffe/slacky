package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Fatal("must have exactly one argument")
	}

	path := args[1]
	channels, err := os.ReadDir(path)
	if err != nil {
		log.Fatalf("invalid file path: %v", err)
	}

	// users
	usersBytes, err := os.ReadFile(fmt.Sprintf("%s/users.json", path))
	if err != nil {
		log.Fatalf("could not open users file: %v", err)
	}

	var users []InputUser
	err = json.Unmarshal(usersBytes, &users)
	if err != nil {
		log.Fatalf("could not unmarshal users: %v", err)
	}

	usersMap := make(map[string]User)
	for _, user := range users {
		name := user.Profile.DisplayName
		if name == "" {
			name = user.Profile.RealName
		}
		usersMap[user.Id] = User{
			Id:   user.Id,
			Name: name,
		}
	}

	channelsMap := make(map[string][]Message)
	// channels
	for _, channel := range channels {
		if !channel.IsDir() {
			continue
		}

		channelPath := fmt.Sprintf("%s/%s", path, channel.Name())
		messages, err := readChannel(channelPath, usersMap)
		if err != nil {
			log.Fatalf("could not read channel messages: %v", err)
		}

		channelsMap[channel.Name()] = messages

	}

	for path, messages := range channelsMap {
		err = writeChannel(path, messages)
		if err != nil {
			log.Fatalf("could not write channel messages: %v", err)
		}
	}
}

func writeChannel(path string, messages []Message) error {
	bytes, err := json.MarshalIndent(messages, "", "\t")
	if err != nil {
		return fmt.Errorf("could not marshall messages: %w", err)
	}

	// create output/debug/formated folders
	outputFolder := "output"
	err = os.MkdirAll(outputFolder, 0777)
	if err != nil {
		return fmt.Errorf("could not create output folder: %w", err)
	}
	debugFolder := "debug"
	err = os.MkdirAll(fmt.Sprintf("%s/%s", outputFolder, debugFolder), 0777)
	if err != nil {
		return fmt.Errorf("could not create debug folder: %w", err)
	}
	formattedFolder := "formatted"
	err = os.MkdirAll(fmt.Sprintf("%s/%s", outputFolder, formattedFolder), 0777)
	if err != nil {
		return fmt.Errorf("could not create formatted folder: %w", err)
	}

	// write json
	jsonFile, err := os.Create(fmt.Sprintf("%s/debug/%s.json", outputFolder, path))
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	_, err = jsonFile.Write(bytes)
	if err != nil {
		return fmt.Errorf("could not write to file: %w", err)
	}

	// write formatted
	formattedFile, err := os.Create(fmt.Sprintf("%s/formatted/%s.txt", outputFolder, path))
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	writer := bufio.NewWriter(formattedFile)
	for _, msg := range messages {
		month := msg.Time.Month()
		day := msg.Time.Day()
		h, m, _ := msg.Time.Clock()
		formatted := fmt.Sprintf("%02d/%d %02d:%02d %s: %s\n", month, day, h, m, msg.User, msg.Text)
		writer.WriteString(formatted)
		writer.WriteByte('\n')
	}
	writer.Flush()

	return nil
}

func readChannel(path string, users map[string]User) ([]Message, error) {
	days, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("could not open channel %s: %w", path, err)
	}

	var messages []Message
	for _, day := range days {
		dayPath := fmt.Sprintf("%s/%s", path, day.Name())
		dayBytes, err := os.ReadFile(dayPath)
		if err != nil {
			return nil, fmt.Errorf("could not open channel file: %w", err)
		}

		var inputMessages []InputMessage
		err = json.Unmarshal(dayBytes, &inputMessages)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal messages: %w", err)
		}

		for _, inputMessage := range inputMessages {
			notMessage := inputMessage.Type != "message"
			isChannelJoin := inputMessage.SubType == "channel_join"
			isReply := inputMessage.ParentUserId != ""

			if notMessage || isChannelJoin || isReply {
				continue
			}

			// replace @ID with actual name
			regex := regexp.MustCompile(`<(@[^<>@]*)>`)
			matches := regex.FindAllStringSubmatch(inputMessage.Text, 999)

			user := users[inputMessage.UserId].Name
			text := inputMessage.Text
			for _, match := range matches {
				id := match[1][1:]
				name := users[id].Name
				text = strings.ReplaceAll(text, match[0], "@"+name)
			}
			tsFloat, err := strconv.ParseFloat(inputMessage.Timestamp, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid timestamp: %w", err)
			}
			ts := time.Unix(int64(tsFloat), 0)

			// remove subteam bloat
			regex = regexp.MustCompile(`<!subteam[^@]*([^>]*)>`)
			matches = regex.FindAllStringSubmatch(text, 999)
			for _, match := range matches {
				text = strings.Replace(text, match[0], match[1], 1)
			}

			// remove channel bloat
			regex = regexp.MustCompile(`<#.*\|(.*)>`)
			matches = regex.FindAllStringSubmatch(text, 999)
			for _, match := range matches {
				text = strings.Replace(text, match[0], "#"+match[1], 1)
			}

			messages = append(messages, Message{
				User: user,
				Text: text,
				Time: ts,
			})
		}
	}

	return messages, nil
}

type User struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Message struct {
	User string    `json:"user"`
	Text string    `json:"text"`
	Time time.Time `json:"time"`
}

type InputUser struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Profile struct {
		DisplayName string `json:"display_name"`
		RealName    string `json:"real_name"`
	} `json:"profile"`
}

type InputMessage struct {
	UserId       string `json:"user"`
	Timestamp    string `json:"ts"`
	Type         string `json:"type"`
	SubType      string `json:"subtype"`
	Text         string `json:"text"`
	ParentUserId string `json:"parent_user_id"`
}
