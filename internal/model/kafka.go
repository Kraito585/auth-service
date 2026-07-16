package model

// KafkaMessage представляет входящий запрос от HTTP handler для отправки в Kafka
type KafkaMessage struct {
	Topic     string `json:"topic"`
	Key       string `json:"key"`
	Payload   string `json:"payload"`
	Operation string `json:"operation"`
	UserUUID  string `json:"user_uuid"`
	RequestID string `json:"request_id"`
}
