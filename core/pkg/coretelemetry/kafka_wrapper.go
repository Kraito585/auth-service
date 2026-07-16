package coretelemetry

import (
	"time"
)

// ObserveKafka выполняет функцию и замеряет время
func ObserveKafka(operation string, fn func() error) error {
	start := time.Now()

	err := fn()

	DependencyDuration.WithLabelValues("kafka", operation).Observe(time.Since(start).Seconds())

	return err
}
