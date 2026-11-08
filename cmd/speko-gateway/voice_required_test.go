package main

import (
	"context"
	"errors"
	"testing"

	"github.com/SpekoAI/gateway/gateway"
	"github.com/SpekoAI/gateway/protocol"
	"github.com/SpekoAI/gateway/providers/alibaba"
	"github.com/SpekoAI/gateway/providers/cartesia"
	"github.com/SpekoAI/gateway/providers/elevenlabs"
	"github.com/SpekoAI/gateway/providers/inworld"
	"github.com/SpekoAI/gateway/providers/minimax"
	"github.com/SpekoAI/gateway/providers/smallest"
	"github.com/SpekoAI/gateway/providers/soniox"
	runtimepkg "github.com/SpekoAI/gateway/runtime"
)

// Every catalog TTS row without a DefaultVoice must be classified: the adapter
// picks a voice itself, or it refuses with the typed error the server
// turns into 422 voice_required. An untyped refusal reaches the caller as a
// bare, retryable session_open_failed.
func TestCatalogTTSRowsWithoutAVoiceAreClassified(t *testing.T) {
	picksItsOwnVoice := map[string]bool{"deepgram.tts.v1": true, "openai.tts.v1": true, "xai.tts.v1": true}
	refusing := map[string]func() (runtimepkg.Adapter, error){
		"alibaba.tts.v1":         func() (runtimepkg.Adapter, error) { return alibaba.NewTTS(alibaba.TTSConfig{}) },
		"cartesia.tts.v1":        func() (runtimepkg.Adapter, error) { return cartesia.New(cartesia.Config{}) },
		"elevenlabs.tts.v1":      func() (runtimepkg.Adapter, error) { return elevenlabs.New(elevenlabs.Config{}) },
		"elevenlabs.dialogue.v1": func() (runtimepkg.Adapter, error) { return elevenlabs.NewDialogue(elevenlabs.Config{}) },
		"inworld.tts.v2":         func() (runtimepkg.Adapter, error) { return inworld.New(inworld.Config{}) },
		"minimax.tts.v1":         func() (runtimepkg.Adapter, error) { return minimax.New(minimax.Config{}) },
		"smallest.tts.v1":        func() (runtimepkg.Adapter, error) { return smallest.New(smallest.Config{}) },
		"soniox.tts.v1":          func() (runtimepkg.Adapter, error) { return soniox.NewTTS(soniox.TTSConfig{}) },
	}
	// A cancelled context keeps a wrongly classified adapter off the network.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, entry := range gateway.Catalog() {
		if entry.Kind != protocol.SessionKindTTS || entry.DefaultVoice != "" || picksItsOwnVoice[entry.Adapter] {
			continue
		}
		t.Run(entry.Adapter, func(t *testing.T) {
			newAdapter, ok := refusing[entry.Adapter]
			if !ok {
				t.Fatalf("%s has no DefaultVoice: give it one, or classify it in this test", entry.Adapter)
			}
			adapter, err := newAdapter()
			if err != nil {
				t.Fatalf("new adapter: %v", err)
			}
			_, err = adapter.Open(ctx, runtimepkg.AdapterRequest{
				Kind:    protocol.SessionKindTTS,
				Options: protocol.RequestOptions{Language: "en"},
				Media:   &protocol.MediaFormat{Encoding: "pcm_s16le", SampleRateHz: 24000, Channels: 1},
				Plan: protocol.SessionPlan{
					Execution: protocol.Execution{ProviderRoute: protocol.RouteProviderDirect, CredentialSource: protocol.CredentialsBYOK},
					Route: protocol.PlanRoute{
						Provider: entry.Provider, Model: entry.DefaultModel, Adapter: entry.Adapter, Transport: entry.Transport, Endpoint: entry.Endpoint,
						Credential: &protocol.DelegatedCredential{Kind: protocol.CredentialBearer, Value: "test-key"},
					},
				},
			})
			var voiceError *protocol.VoiceRequiredError
			if !errors.As(err, &voiceError) || voiceError.Provider != entry.Provider {
				t.Fatalf("open with no voice = %v, want VoiceRequiredError for %s", err, entry.Provider)
			}
		})
	}
}
