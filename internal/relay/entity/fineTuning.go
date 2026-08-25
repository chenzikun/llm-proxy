package entity

type FineTuningHyperParameter struct {
	BatchSize              any `json:"batch_size,omitempty" binding:"Optional"`
	NumEpochs              int `json:"num_epochs,default=1"`
	LearningRateMultiplier any `json:"learning_rate_multiplier,omitempty" binding:"Optional"`
}

type FineTuningWandb struct {
	Project string   `json:"project" binding:"required"`
	Name    string   `json:"name,omitempty" binding:"Optional"`
	Entity  string   `json:"entity,omitempty" binding:"Optional"`
	Tags    []string `json:"tags,omitempty" binding:"Optional"`
}

type FineTuningIntegration struct {
	Type  string          `json:"type" binding:"required"`
	Wandb FineTuningWandb `json:"wandb" binding:"required"`
}

type FineTuningRequest struct {
	Model           string                   `json:"model"`
	TrainingFile    string                   `json:"training_file" binding:"required"`
	Suffix          string                   `json:"suffix,omitempty" binding:"Optional"`
	Seed            int                      `json:"seed,omitempty" binding:"Optional"`
	ValidationFile  string                   `json:"validation_file,omitempty" binding:"Optional"`
	HyperParameters FineTuningHyperParameter `json:"hyper_parameters,omitempty" binding:"required"`
	Integrations    []FineTuningIntegration  `json:"integrations,omitempty" binding:"Optional"`
}

type HyperparamResponse struct {
	NEpochs                int     `json:"n_epochs"`
	BatchSize              int     `json:"batch_size"`
	LearningRateMultiplier float64 `json:"learning_rate_multiplier"`
}

type FineTuningResponse struct {
	Object          string             `json:"object"`
	Id              string             `json:"id"`
	Model           string             `json:"model"`
	CreatedAt       int                `json:"created_at"`
	FinishedAt      int                `json:"finished_at"`
	FineTunedModel  string             `json:"fine_tuned_model"`
	OrganizationId  string             `json:"organization_id"`
	ResultFiles     []string           `json:"result_files"`
	Status          string             `json:"status"`
	ValidationFile  string             `json:"validation_file"`
	TrainingFile    string             `json:"training_file"`
	TrainedTokens   int                `json:"trained_tokens"`
	Hyperparameters HyperparamResponse `json:"hyperparameters"`
}
