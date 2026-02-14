package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"TestTaskJustPay/config"
	"TestTaskJustPay/pkg/logger"

	"github.com/segmentio/kafka-go"
)

const commitTimeout = 5 * time.Second

func Run(cfg config.AnalyticsConfig) {
	logger.Setup(logger.Options{
		Level:   cfg.LogLevel,
		Console: strings.ToLower(os.Getenv("LOG_FORMAT")) == "console",
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.Info("Starting Analytics consumer",
		"topic", cfg.KafkaEventsTopic,
		"group", cfg.KafkaConsumerGroup,
		"brokers", cfg.KafkaBrokers,
		"index", cfg.OpensearchIndex,
	)

	idx, err := newIndexer(cfg.OpensearchURLs, cfg.OpensearchIndex)
	if err != nil {
		slog.Error("Failed to create OpenSearch indexer", slog.Any("error", err))
		os.Exit(1)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:          cfg.KafkaBrokers,
		Topic:            cfg.KafkaEventsTopic,
		GroupID:          cfg.KafkaConsumerGroup,
		MinBytes:         1,
		MaxBytes:         10e6, // 10MB
		CommitInterval:   0,    // manual commit
		StartOffset:      kafka.FirstOffset,
		MaxWait:          500 * time.Millisecond,
		RebalanceTimeout: 5 * time.Second,
	})
	defer func() {
		if err := reader.Close(); err != nil {
			slog.Error("Failed to close Kafka reader", slog.Any("error", err))
		}
	}()

	consumeLoop(ctx, reader, idx)

	slog.Info("Analytics consumer stopped")
}

func consumeLoop(ctx context.Context, reader *kafka.Reader, idx *indexer) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			slog.Error("Failed to fetch message", slog.Any("error", err))
			return
		}

		slog.Debug("Message received",
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
			"key", string(msg.Key),
		)

		var evt event
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			slog.Error("Failed to unmarshal event, skipping",
				"offset", msg.Offset,
				slog.Any("error", err),
			)
			commitMessage(reader, msg)
			continue
		}

		evt.CreatedAt = normalizeTimestamp(evt.CreatedAt)

		if err := idx.indexEvent(ctx, evt); err != nil {
			slog.Error("Failed to index event, message not committed",
				"event_id", evt.ID,
				"aggregate_type", evt.AggregateType,
				"aggregate_id", evt.AggregateID,
				slog.Any("error", err),
			)
			// Don't commit — Kafka will redeliver on restart
			continue
		}

		slog.Debug("Event indexed",
			"event_id", evt.ID,
			"aggregate_type", evt.AggregateType,
			"aggregate_id", evt.AggregateID,
			"event_type", evt.EventType,
		)

		commitMessage(reader, msg)
	}
}

func commitMessage(reader *kafka.Reader, msg kafka.Message) {
	commitCtx, cancel := context.WithTimeout(context.Background(), commitTimeout)
	defer cancel()

	if err := reader.CommitMessages(commitCtx, msg); err != nil {
		slog.Error("Failed to commit message",
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
			slog.Any("error", err),
		)
	}
}
