package commands

import (
	"context"
	"fmt"

	"tg_bot/internal/database"
	"tg_bot/internal/telegram"
)

func recordAction(ctx context.Context, st database.Store, msg *telegram.Message, action string, target *telegram.Message) error {
	if msg == nil || msg.Chat == nil {
		return fmt.Errorf("message or chat is nil")
	}
	record := database.ActionRecord{
		ChatID:    msg.Chat.ID,
		MessageID: msg.MessageID,
		Action:    action,
		EventType: "moderation",
	}
	if msg.From != nil {
		record.ActorID = msg.From.ID
		record.ActorFirstName = msg.From.FirstName
	}
	if target != nil {
		record.TargetMessageID = target.MessageID
		if target.From != nil {
			record.TargetUserID = target.From.ID
			record.TargetUsername = target.From.Username
		}
	}
	return st.RecordAction(ctx, record)
}
