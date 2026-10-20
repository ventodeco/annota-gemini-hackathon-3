package ai

// routedClient preserves the application interface while composing protocol-specific tasks.
type routedClient struct {
	*chatTasks
	*speechTask
}

var _ Client = (*routedClient)(nil)

func NewRoutedClient(cfg RoutingConfig, opts RoutedClientOptions) (Client, error) {
	if err := ValidateRoutingConfig(cfg); err != nil {
		return nil, err
	}
	return &routedClient{
		chatTasks:  &chatTasks{ocr: newTaskGateway(cfg, cfg.Tasks.OCR, "ocr", opts), annotation: newTaskGateway(cfg, cfg.Tasks.Annotation, "annotation", opts)},
		speechTask: &speechTask{gateway: newTaskGateway(cfg, cfg.Tasks.Speech, "speech", opts)},
	}, nil
}
