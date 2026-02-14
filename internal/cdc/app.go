package cdc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"TestTaskJustPay/config"
	"TestTaskJustPay/pkg/logger"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/segmentio/kafka-go"
)

const (
	standbyTimeout = 10 * time.Second
	retryBaseDelay = 2 * time.Second
	retryMaxDelay  = 30 * time.Second
)

func Run(cfg config.CDCConfig) {
	logger.Setup(logger.Options{
		Level:   cfg.LogLevel,
		Console: strings.ToLower(os.Getenv("LOG_FORMAT")) == "console",
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.Info("Starting CDC worker",
		"slot", cfg.SlotName,
		"publication", cfg.PublicationName,
		"topic", cfg.KafkaEventsTopic,
		"brokers", cfg.KafkaBrokers,
	)

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.KafkaBrokers...),
		Topic:        cfg.KafkaEventsTopic,
		Balancer:     &kafka.Hash{},
		BatchTimeout: 10 * time.Millisecond, // low latency; default 1s adds per-write delay
		RequiredAcks: kafka.RequireOne,
	}
	defer func() {
		if err := writer.Close(); err != nil {
			slog.Error("Failed to close Kafka writer", slog.Any("error", err))
		}
	}()

	connStr := cfg.PgURL
	if strings.Contains(connStr, "?") {
		connStr += "&replication=database"
	} else {
		connStr += "?replication=database"
	}

	// Retry loop: the publication or table may not exist yet when CDC starts
	// (API service creates them via migrations). We retry with backoff until
	// the replication stream is established.
	delay := retryBaseDelay
	for {
		err := runReplication(ctx, connStr, cfg.SlotName, cfg.PublicationName, writer)
		if err == nil || ctx.Err() != nil {
			break
		}
		slog.Warn("Replication failed, retrying...", "error", err, "delay", delay)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			break
		}
		delay = min(delay*2, retryMaxDelay)
	}

	slog.Info("CDC worker stopped")
}

// runReplication connects to PG, sets up the replication slot, and streams
// until ctx is cancelled or an unrecoverable error occurs.
// Returns nil on clean shutdown, error if we should retry.
func runReplication(ctx context.Context, connStr, slotName, publication string, writer *kafka.Writer) error {
	conn, err := pgconn.Connect(ctx, connStr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(context.Background())

	// Drop the slot if it exists and recreate fresh. This ensures we always
	// start from the current WAL position with a clean state. A stale slot
	// (created before the publication existed) causes "publication does not
	// exist" errors even though the publication is present in the catalog.
	//
	// Trade-off: on restart we miss events that happened while the worker was
	// down. This is acceptable for a log-only listener; when we add Kafka
	// publishing, we'll persist the LSN externally and reuse the slot.
	_ = pglogrepl.DropReplicationSlot(ctx, conn, slotName, pglogrepl.DropReplicationSlotOptions{})

	_, err = pglogrepl.CreateReplicationSlot(
		ctx, conn, slotName, "pgoutput",
		pglogrepl.CreateReplicationSlotOptions{
			Mode: pglogrepl.LogicalReplication,
		},
	)
	if err != nil {
		return fmt.Errorf("create slot: %w", err)
	}
	slog.Info("Replication slot created", "slot", slotName)

	sysident, err := pglogrepl.IdentifySystem(ctx, conn)
	if err != nil {
		return fmt.Errorf("identify system: %w", err)
	}
	slog.Info("System identified",
		"systemid", sysident.SystemID,
		"timeline", sysident.Timeline,
		"xlogpos", sysident.XLogPos,
		"dbname", sysident.DBName,
	)

	// Start streaming from the slot's confirmed position (LSN 0 means
	// "resume from where the slot left off").
	err = pglogrepl.StartReplication(
		ctx, conn, slotName, pglogrepl.LSN(0),
		pglogrepl.StartReplicationOptions{
			PluginArgs: []string{
				"proto_version '1'",
				fmt.Sprintf("publication_names '%s'", publication),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("start replication: %w", err)
	}
	slog.Info("Streaming started")

	return streamLoop(ctx, conn, writer)
}

// streamLoop receives WAL messages until ctx is cancelled or connection breaks.
// Returns nil on clean shutdown, error if we should reconnect.
func streamLoop(ctx context.Context, conn *pgconn.PgConn, writer *kafka.Writer) error {
	relations := make(map[uint32]*pglogrepl.RelationMessage)
	var clientXLogPos pglogrepl.LSN

	nextStandbyDeadline := time.Now().Add(standbyTimeout)

	for {
		if time.Now().After(nextStandbyDeadline) {
			err := pglogrepl.SendStandbyStatusUpdate(
				ctx, conn,
				pglogrepl.StandbyStatusUpdate{WALWritePosition: clientXLogPos},
			)
			if err != nil {
				return fmt.Errorf("standby status: %w", err)
			}
			nextStandbyDeadline = time.Now().Add(standbyTimeout)
		}

		receiveCtx, receiveCancel := context.WithDeadline(ctx, nextStandbyDeadline)
		rawMsg, err := conn.ReceiveMessage(receiveCtx)
		receiveCancel()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil // graceful shutdown
			}
			if pgconn.Timeout(err) {
				continue // deadline expired — send heartbeat
			}
			return fmt.Errorf("receive: %w", err)
		}

		switch msg := rawMsg.(type) {
		case *pgproto3.CopyData:
			switch msg.Data[0] {
			case pglogrepl.PrimaryKeepaliveMessageByteID:
				pkm, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
				if err != nil {
					slog.Error("ParsePrimaryKeepaliveMessage failed", slog.Any("error", err))
					continue
				}
				if pkm.ReplyRequested {
					nextStandbyDeadline = time.Time{} // force immediate heartbeat
				}

			case pglogrepl.XLogDataByteID:
				xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
				if err != nil {
					slog.Error("ParseXLogData failed", slog.Any("error", err))
					continue
				}

				if err := handleWALMessage(ctx, xld.WALData, relations, writer); err != nil {
					return fmt.Errorf("handle WAL message: %w", err)
				}

				clientXLogPos = xld.WALStart + pglogrepl.LSN(len(xld.WALData))
			}

		case *pgproto3.ErrorResponse:
			// PG sends ErrorResponse when replication fails (e.g. publication
			// was dropped, or didn't exist when stream started).
			return fmt.Errorf("server error: %s (code %s)", msg.Message, msg.Code)

		default:
			slog.Debug("Ignoring message", "type", fmt.Sprintf("%T", rawMsg))
		}
	}
}

func handleWALMessage(ctx context.Context, walData []byte, relations map[uint32]*pglogrepl.RelationMessage, writer *kafka.Writer) error {
	msg, err := pglogrepl.Parse(walData)
	if err != nil {
		return fmt.Errorf("parse WAL message: %w", err)
	}

	switch m := msg.(type) {
	case *pglogrepl.RelationMessage:
		relations[m.RelationID] = m
		slog.Debug("RelationMessage",
			"relation_id", m.RelationID,
			"namespace", m.Namespace,
			"name", m.RelationName,
		)

	case *pglogrepl.InsertMessage:
		rel, ok := relations[m.RelationID]
		if !ok {
			slog.Warn("Unknown relation in InsertMessage", "relation_id", m.RelationID)
			return nil
		}

		evt, err := decodeInsert(rel, m)
		if err != nil {
			return fmt.Errorf("decode insert: %w", err)
		}

		value, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}

		if err := writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(evt.AggregateID),
			Value: value,
		}); err != nil {
			return fmt.Errorf("publish to kafka: %w", err)
		}

		slog.Debug("Event published",
			"aggregate_type", evt.AggregateType,
			"aggregate_id", evt.AggregateID,
			"event_type", evt.EventType,
		)

	case *pglogrepl.BeginMessage:
		slog.Debug("BEGIN", "xid", m.Xid, "lsn", m.FinalLSN)

	case *pglogrepl.CommitMessage:
		slog.Debug("COMMIT", "lsn", m.CommitLSN)
	}

	return nil
}

func isPgError(err error, code string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}
	return false
}
