package dto

type SystemSettingsResponse struct {
	Settings map[string]string `json:"settings"`
}

type UpdateSettingRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PingResponse struct {
	Status string `json:"status"`
}

type CustomModelsRequest struct {
	Provider string  `json:"provider"`
	APIKey   *string `json:"apiKey"`
	BasePath *string `json:"basePath"`
}
