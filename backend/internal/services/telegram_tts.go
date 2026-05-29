package services

import (
	"context"

	"github.com/odysseythink/mlog"
)

func (s *TelegramBotService) maybeSendVoiceReply(chatID int64, text string, forceVoice bool) {
	if s.ttsProvider == nil || !s.ttsProvider.Available() {
		return
	}
	if !forceVoice {
		s.mu.RLock()
		mode := ""
		if s.config != nil {
			mode = s.config.VoiceResponseMode
		}
		s.mu.RUnlock()
		if mode == "text_only" {
			return
		}
		// mirror mode: only voice if user sent voice (caller must set forceVoice)
		if mode == "mirror" && !forceVoice {
			return
		}
	}

	go func() {
		synth, err := s.ttsProvider.Synthesize(context.Background(), text)
		if err != nil {
			mlog.Warning("telegram tts failed: ", err)
			return
		}
		_ = s.sendVoice(chatID, synth.Audio, synth.ContentType)
	}()
}

func (s *TelegramBotService) sendVoice(chatID int64, audio []byte, contentType string) error {
	// Telegram voice messages require .ogg with Opus. If TTS returns MP3, we send as audio document.
	// For now, send as audio document regardless.
	_ = audio
	_ = contentType
	_ = chatID
	// TODO(PR5-followup): use tgbotapi.NewAudioShare or tgbotapi.NewVoiceUpload
	// See plan: .gpowers/plans/2026-05-28-telegram-integration.md
	mlog.Info("telegram: would send voice message, length=", len(audio))
	return nil
}
