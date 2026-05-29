package router

import (
	"sort"
	"strings"
	"time"

	"proxy-hub/internal/config"
)

type Index struct {
	aliasToHits map[string][]Hit
	passthrough []Hit
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
		baseHit := Hit{
			ChannelName:   ch.Name,
			ChannelType:   config.ChannelTypeOpenAIAPI,
			BaseURL:       ch.BaseURL,
			Priority:      ch.EffectivePriority(),
			Timeout:       time.Duration(ch.EffectiveTimeoutSec()) * time.Second,
			APIKeyEntries: append([]config.APIKeyEntry(nil), ch.APIKeyEntries...),
		}
		idx.passthrough = append(idx.passthrough, baseHit)
		for _, model := range ch.Models {
			upstreamModel := strings.TrimSpace(model.Name)
			if upstreamModel == "" {
				continue
			}
			displayAlias := strings.TrimSpace(model.EffectiveAlias())
			aliasKey := normalizeAlias(displayAlias)
			if aliasKey == "" {
				continue
			}
			idx.displayAlias(aliasKey, displayAlias)
			hit := baseHit
			hit.UpstreamModelName = upstreamModel
			idx.aliasToHits[aliasKey] = append(idx.aliasToHits[aliasKey], hit)
		}
	}
	for _, ch := range cfg.ChatGPTOAuth {
		if ch.Disabled {
			continue
		}
		for _, model := range ch.Models {
			upstreamModel := strings.TrimSpace(model.Name)
			if upstreamModel == "" {
				continue
			}
			displayAlias := strings.TrimSpace(model.EffectiveAlias())
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
				UpstreamModelName: upstreamModel,
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
	requested := strings.TrimSpace(alias)
	aliasKey := normalizeAlias(requested)
	if aliasKey == "" {
		return nil
	}
	hits := i.aliasToHits[aliasKey]
	if len(hits) > 0 {
		return cloneHits(hits)
	}
	return i.passthroughHits(requested)
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

func (i *Index) passthroughHits(model string) []Hit {
	if len(i.passthrough) == 0 {
		return nil
	}
	hits := make([]Hit, 0, len(i.passthrough))
	for _, hit := range i.passthrough {
		hit.UpstreamModelName = model
		hit.APIKeyEntries = append([]config.APIKeyEntry(nil), hit.APIKeyEntries...)
		hits = append(hits, hit)
	}
	return hits
}

func cloneHits(hits []Hit) []Hit {
	out := make([]Hit, len(hits))
	for i, hit := range hits {
		hit.APIKeyEntries = append([]config.APIKeyEntry(nil), hit.APIKeyEntries...)
		out[i] = hit
	}
	return out
}

func normalizeAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}
