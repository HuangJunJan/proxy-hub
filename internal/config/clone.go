package config

func Clone(in *Config) *Config {
	if in == nil {
		return &Config{}
	}
	out := *in
	if in.Admin != nil {
		admin := *in.Admin
		out.Admin = &admin
	}
	out.APIKeys = append([]APIKeyConfig(nil), in.APIKeys...)
	out.OpenAIAPI = cloneOpenAIChannels(in.OpenAIAPI)
	out.ChatGPTOAuth = cloneOAuthChannels(in.ChatGPTOAuth)
	out.CORS.AllowedOrigins = append([]string(nil), in.CORS.AllowedOrigins...)
	return &out
}

func cloneOpenAIChannels(in []OpenAIAPIChannel) []OpenAIAPIChannel {
	out := make([]OpenAIAPIChannel, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].APIKeyEntries = append([]APIKeyEntry(nil), in[i].APIKeyEntries...)
		out[i].Models = append([]ModelEntry(nil), in[i].Models...)
	}
	return out
}

func cloneOAuthChannels(in []ChatGPTOAuthChannel) []ChatGPTOAuthChannel {
	out := make([]ChatGPTOAuthChannel, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Models = append([]ModelEntry(nil), in[i].Models...)
	}
	return out
}
