package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/channels"
	slackapi "github.com/slack-go/slack"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
)

// Send delivers an outbound message to Slack.
func (c *Channel) Send(_ context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("slack bot not running")
	}

	channelID := msg.ChatID
	if channelID == "" {
		return fmt.Errorf("empty chat ID for slack send")
	}

	placeholderKey := channelID
	if pk := msg.Metadata["placeholder_key"]; pk != "" {
		placeholderKey = pk
	}
	threadTS := msg.Metadata["message_thread_id"]

	// DEBUG
	keys := make([]string, 0, len(msg.Metadata))
	for k := range msg.Metadata {
		keys = append(keys, k)
	}
	slog.Debug("slack send called",
		"channel_id", channelID,
		"placeholder_key", placeholderKey,
		"thread_ts", threadTS,
		"content_preview", channels.Truncate(msg.Content, 50),
		"content_len", len(msg.Content),
		"placeholder_update", msg.Metadata["placeholder_update"],
		"stream_finalized", msg.Metadata["stream_finalized"],
		"metadata_keys", fmt.Sprintf("%v", keys),
	)

	// Extract bare Slack channel ID from composite localKey (e.g. "C0123:thread:TS" → "C0123").
	// The composite localKey is used as chatID for routing but Slack API calls require bare channel IDs.
	slackChannelID := extractBareChannelID(channelID)

	// Placeholder update (LLM retry notification)
	if msg.Metadata["placeholder_update"] == "true" {
		if pTS, ok := c.placeholders.Load(placeholderKey); ok {
			ts := pTS.(string)
			_, _, _, _ = c.api.UpdateMessage(slackChannelID, ts,
				slackapi.MsgOptionText(msg.Content, false))
		}
		return nil
	}

	content := msg.Content

	// NO_REPLY: delete placeholder, return
	// But skip deletion if stream_finalized=true — the streaming pipeline already has the
	// placeholder TS and FinalizeStream handed it off; deleting would remove streamed content.
	if content == "" {
		if msg.Metadata["stream_finalized"] != "true" {
			if pTS, ok := c.placeholders.Load(placeholderKey); ok {
				c.placeholders.Delete(placeholderKey)
				ts := pTS.(string)
				_, _, _ = c.api.DeleteMessage(slackChannelID, ts)
			}
		}
		return nil
	}

	content = markdownToSlackMrkdwn(content)

	// Edit placeholder with first chunk, send rest as follow-ups
	if pTS, ok := c.placeholders.Load(placeholderKey); ok {
		c.placeholders.Delete(placeholderKey)
		ts := pTS.(string)

		editContent, remaining := splitAtLimit(content, maxMessageLen)

		opts := []slackapi.MsgOption{slackapi.MsgOptionText(editContent, false)}
		if threadTS != "" {
			opts = append(opts, slackapi.MsgOptionTS(threadTS))
		}

		if _, _, _, editErr := c.api.UpdateMessage(slackChannelID, ts, opts...); editErr == nil {
			if remaining != "" {
				return c.sendChunked(slackChannelID, remaining, threadTS)
			}
			return nil
		} else {
			slog.Warn("slack placeholder edit failed, sending new message",
				"channel_id", slackChannelID, "error", editErr)
		}
	}

	// Handle media attachments
	for _, media := range msg.Media {
		if err := c.uploadFile(slackChannelID, threadTS, media); err != nil {
			slog.Warn("slack: file upload failed",
				"file", media.URL, "error", err)
			c.sendChunked(slackChannelID, fmt.Sprintf("[File upload failed: %s]", media.URL), threadTS)
		}
	}

	return c.sendChunked(slackChannelID, content, threadTS)
}

// sendChunked sends message chunks using markdown-aware splitting.
func (c *Channel) sendChunked(channelID, content, threadTS string) error {
	for _, chunk := range channels.ChunkMarkdown(content, maxMessageLen) {
		opts := []slackapi.MsgOption{slackapi.MsgOptionText(chunk, false)}
		if threadTS != "" {
			opts = append(opts, slackapi.MsgOptionTS(threadTS))
		}

		if _, _, err := c.api.PostMessage(channelID, opts...); err != nil {
			return fmt.Errorf("send slack message: %w", err)
		}
	}
	return nil
}

// extractBareChannelID extracts the bare Slack channel ID from a composite localKey.
// Handles formats: "C0123:thread:TS" or "C0123:topic:ID" → returns "C0123".
// If no composite suffix is found, returns the key unchanged.
func extractBareChannelID(key string) string {
	if idx := strings.Index(key, ":thread:"); idx != -1 {
		return key[:idx]
	}
	if idx := strings.Index(key, ":topic:"); idx != -1 {
		return key[:idx]
	}
	return key
}

// splitAtLimit splits content into first chunk + remaining using markdown-aware chunking.
func splitAtLimit(content string, maxLen int) (chunk, remaining string) {
	chunks := channels.ChunkMarkdown(content, maxLen)
	if len(chunks) == 0 {
		return "", ""
	}
	if len(chunks) == 1 {
		return chunks[0], ""
	}
	return chunks[0], strings.Join(chunks[1:], "\n")
}
