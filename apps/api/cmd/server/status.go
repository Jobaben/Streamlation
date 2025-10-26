package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	statuspkg "streamlation/packages/backend/status"

	"go.uber.org/zap"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type StatusSubscriber interface {
	Subscribe(ctx context.Context, sessionID string) (statuspkg.StatusStream, error)
}

func sessionStatusHandler(subscriber StatusSubscriber, logger *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sessionID := r.PathValue("id")
		if !sessionIDPattern.MatchString(sessionID) {
			writeError(w, logger, http.StatusBadRequest, fmt.Errorf("invalid session id"))
			return
		}

		if !strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") || strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
			http.Error(w, "websocket upgrade required", http.StatusBadRequest)
			return
		}

		key := r.Header.Get("Sec-WebSocket-Key")
		if key == "" {
			http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
			return
		}

		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "websocket not supported", http.StatusInternalServerError)
			return
		}

		conn, rw, err := hj.Hijack()
		if err != nil {
			logger.Errorw("failed to hijack connection", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		acceptKey := computeAcceptKey(key)
		response := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", acceptKey)
		if _, err := rw.WriteString(response); err != nil {
			_ = conn.Close()
			logger.Errorw("failed to write websocket handshake", "error", err)
			return
		}
		if err := rw.Flush(); err != nil {
			_ = conn.Close()
			logger.Errorw("failed to flush websocket handshake", "error", err)
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		stream, err := subscriber.Subscribe(ctx, sessionID)
		if err != nil {
			logger.Errorw("failed to subscribe to status stream", "error", err, "sessionID", sessionID)
			_ = writeWebSocketCloseFrame(conn, 1011)
			_ = conn.Close()
			return
		}
		defer func() {
			if err := stream.Close(); err != nil {
				logger.Errorw("failed to close status stream", "error", err, "sessionID", sessionID)
			}
			_ = writeWebSocketCloseFrame(conn, 1000)
			_ = conn.Close()
		}()

		go websocketReadLoop(ctx, conn, cancel, logger)

		for {
			select {
			case event, ok := <-stream.Events():
				if !ok {
					return
				}
				payload, err := json.Marshal(event)
				if err != nil {
					logger.Errorw("failed to marshal status event", "error", err, "sessionID", sessionID)
					continue
				}
				if err := writeWebSocketTextFrame(conn, payload); err != nil {
					logger.Errorw("failed to write status event", "error", err, "sessionID", sessionID)
					return
				}
			case err, ok := <-stream.Errors():
				if ok && err != nil {
					logger.Errorw("status stream error", "error", err, "sessionID", sessionID)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func writeWebSocketTextFrame(conn net.Conn, payload []byte) error {
	return writeWebSocketFrame(conn, 0x1, payload)
}

func writeWebSocketCloseFrame(conn net.Conn, code uint16) error {
	payload := make([]byte, 2)
	binary.BigEndian.PutUint16(payload, code)
	return writeWebSocketFrame(conn, 0x8, payload)
}

func writeWebSocketFrame(conn net.Conn, opcode byte, payload []byte) error {
	frame := []byte{0x80 | opcode}
	length := len(payload)
	switch {
	case length <= 125:
		frame = append(frame, byte(length))
	case length <= 65535:
		frame = append(frame, 126, byte(length>>8), byte(length))
	default:
		extended := make([]byte, 8)
		binary.BigEndian.PutUint64(extended, uint64(length))
		frame = append(frame, 127)
		frame = append(frame, extended...)
	}

	frame = append(frame, payload...)
	if _, err := conn.Write(frame); err != nil {
		return err
	}
	return nil
}

func websocketReadLoop(ctx context.Context, conn net.Conn, cancel context.CancelFunc, logger *zap.SugaredLogger) {
	reader := bufio.NewReader(conn)
	for {
		if ctx.Err() != nil {
			return
		}

		if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			logger.Errorw("failed to set websocket read deadline", "error", err)
			cancel()
			return
		}

		first, err := reader.ReadByte()
		if err != nil {
			cancel()
			return
		}

		second, err := reader.ReadByte()
		if err != nil {
			cancel()
			return
		}

		opcode := first & 0x0F
		payloadLen := int64(second & 0x7F)
		if payloadLen == 126 {
			buf := make([]byte, 2)
			if _, err := io.ReadFull(reader, buf); err != nil {
				cancel()
				return
			}
			payloadLen = int64(binary.BigEndian.Uint16(buf))
		} else if payloadLen == 127 {
			buf := make([]byte, 8)
			if _, err := io.ReadFull(reader, buf); err != nil {
				cancel()
				return
			}
			payloadLen = int64(binary.BigEndian.Uint64(buf))
		}

		masked := second&0x80 != 0
		if masked {
			mask := make([]byte, 4)
			if _, err := io.ReadFull(reader, mask); err != nil {
				cancel()
				return
			}
		}

		if err := discardPayload(reader, payloadLen); err != nil {
			cancel()
			return
		}

		switch opcode {
		case 0x8: // close
			cancel()
			return
		case 0x9: // ping
			if err := writeWebSocketFrame(conn, 0xA, nil); err != nil {
				cancel()
				return
			}
		default:
			continue
		}
	}
}

func discardPayload(r *bufio.Reader, length int64) error {
	if length <= 0 {
		return nil
	}
	buf := make([]byte, 1024)
	remaining := length
	for remaining > 0 {
		chunk := int64(len(buf))
		if remaining < chunk {
			chunk = remaining
		}
		if _, err := io.ReadFull(r, buf[:chunk]); err != nil {
			return err
		}
		remaining -= chunk
	}
	return nil
}
