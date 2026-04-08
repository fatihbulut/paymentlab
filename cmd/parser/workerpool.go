package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/moov-io/iso8583"
)

type jobKind uint8

const (
	jobParse jobKind = iota + 1
	jobPack
)

type job struct {
	kind jobKind
	ctx  context.Context

	rawHex string
	msg    ISOMessage

	replyCh chan jobResult
	start   time.Time
}

type jobResult struct {
	status int
	body   any
	err    error
}

type WorkerPool struct {
	q       chan job
	metrics *Metrics
}

func NewWorkerPool(queueSize, workers int, metrics *Metrics) *WorkerPool {
	wp := &WorkerPool{
		q:       make(chan job, queueSize),
		metrics: metrics,
	}
	for i := 0; i < workers; i++ {
		go wp.worker()
	}
	return wp
}

func (wp *WorkerPool) TrySubmit(j job) bool {
	select {
	case wp.q <- j:
		if wp.metrics != nil {
			wp.metrics.IncQueueDepth()
		}
		return true
	default:
		return false
	}
}

func (wp *WorkerPool) worker() {
	for j := range wp.q {
		if wp.metrics != nil {
			wp.metrics.DecQueueDepth()
		}
		if j.ctx != nil {
			select {
			case <-j.ctx.Done():
				// Client already gave up / request timed out. Drop work early.
				if wp.metrics != nil {
					wp.metrics.IncLateDropped()
				}
				j.replyCh <- jobResult{status: http.StatusGatewayTimeout, body: map[string]any{"error": "timeout"}, err: j.ctx.Err()}
				continue
			default:
			}
		}
		switch j.kind {
		case jobParse:
			j.replyCh <- doParse(j.rawHex)
		case jobPack:
			j.replyCh <- doPack(j.msg)
		default:
			j.replyCh <- jobResult{status: http.StatusInternalServerError, body: map[string]any{"error": "unknown job"}}
		}
	}
}

func doParse(rawHex string) jobResult {
	rawBytes, err := hex.DecodeString(rawHex)
	if err != nil {
		return jobResult{status: http.StatusBadRequest, body: map[string]any{"error": "Hex formatı hatalı"}, err: err}
	}

	message := iso8583.NewMessage(spec)
	if err := message.Unpack(rawBytes); err != nil {
		return jobResult{status: http.StatusUnprocessableEntity, body: map[string]any{"error": fmt.Sprintf("ISO Parse Hatası: %v", err)}, err: err}
	}

	msgData := &ISOMessage{}
	message.Unmarshal(msgData)
	return jobResult{status: http.StatusOK, body: msgData}
}

func doPack(incomingData ISOMessage) jobResult {
	message := iso8583.NewMessage(spec)
	message.MTI(incomingData.MTI)
	if err := message.Marshal(&incomingData); err != nil {
		return jobResult{status: http.StatusInternalServerError, body: map[string]any{"error": "ISO paketleme hazırlığı başarısız"}, err: err}
	}

	rawBytes, err := message.Pack()
	if err != nil {
		return jobResult{status: http.StatusUnprocessableEntity, body: map[string]any{"error": fmt.Sprintf("ISO Pack Hatası: %v", err)}, err: err}
	}

	return jobResult{
		status: http.StatusOK,
		body: map[string]any{
			"status":  "Success",
			"hex":     hex.EncodeToString(rawBytes),
			"length":  len(rawBytes),
			"details": incomingData,
		},
	}
}
