package router

import (
	"sort"
	"strings"
	"time"

	"proxy-hub/internal/config"
)

type Index struct {
	aliasToHits map[string][]Hit
	display     map[string]string
	models      []string
}

type Hit struct {
	ChannelName       string
	ChannelType       string
	BaseURL           string
	Priority          int
	Timeout           time.Duration
	UpstreamModelName string
	APIKeyEntries     []config.APIKeyEntry
}

func NewIndex(cfg *config.Config) *Index {
	idx := &Index{aliasToHits: map[string][]Hit{}, display: map[string]string{}}
	if cfg == nil {
		return idx
	}
	for _, ch := range cfg.OpenAIAPI {
		if ch.Disabled {
			continue
		}
		for _, model := range ch.Models {
			displayAlias := model.EffectiveAlias()
			aliasKey := normalizeAlias(displayAlias)
			if aliasKey == "" {
				continue
			}
			idx.displayAlias(aliasKey, displayAlias)
			idx.aliasToHits[aliasKey] = append(idx.aliasToHits[aliasKey], Hit{
				ChannelName:       ch.Name,
				ChannelType:       config.ChannelTypeOpenAIAPI,
				BaseURL:           ch.BaseURL,
				Priority:          ch.EffectivePriority(),
				Timeout:           time.Duration(ch.EffectiveTimeoutSec()) * time.Second,
				UpstreamModelName: strings.TrimSpace(model.Name),
				APIKeyEntries:     append([]config.APIKeyEntry(nil), ch.APIKeyEntries...),
			})
		}
	}
	for _, ch := range cfg.ChatGPTOAuth {
		if ch.Disabled {
			continue
		}
		for _, model := range ch.Models {
			displayAlias := model.EffectiveAlias()
			aliasKey := normalizeAlias(displayAlias)
			if aliasKey == "" {
				continue
			}
			idx.displayAlias(aliasKey, displayAlias)
			idx.aliasToHits[aliasKey] = append(idx.aliasToHits[aliasKey], Hit{
				ChannelName:       ch.Name,
				ChannelType:       config.ChannelTypeChatGPTOAuth,
				Priority:          config.DefaultPriority,
				Timeout:           time.Duration(ch.EffectiveTimeoutSec()) * time.Second,
				UpstreamModelName: strings.TrimSpace(model.Name),
			})
		}
	}
	idx.models = idx.sortedModels()
	return idx
}

func (i *Index) Resolve(alias string) []Hit {
	if i == nil {
		return nil
	}
	hits := i.aliasToHits[normalizeAlias(alias)]
	return append([]Hit(nil), hits...)
}

func (i *Index) Models() []string {
	if i == nil {
		return nil
	}
	return append([]string(nil), i.models...)
}

func (i *Index) sortedModels() []string {
	models := make([]string, 0, len(i.aliasToHits))
	for alias := range i.aliasToHits {
		display := i.display[alias]
		if display == "" {
			display = alias
		}
		models = append(models, display)
	}
	sort.Strings(models)
	return models
}

func (i *Index) displayAlias(key, display string) {
	if _, ok := i.display[key]; !ok {
		i.display[key] = display
	}
}

func normalizeAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}
