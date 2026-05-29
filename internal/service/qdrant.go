package service

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"media2rag/internal/config"
	"media2rag/internal/llm"
	"media2rag/internal/store"
)

const (
	pollInterval = 500 * time.Millisecond
	pollTimeout  = 30 * time.Second
)

type Qdrant struct {
	cfg          config.QdrantConfig
	st           store.VectorStore
	ownContainer bool
}

func NewQdrant(cfg config.QdrantConfig) *Qdrant {
	return &Qdrant{cfg: cfg}
}

func (q *Qdrant) EnsureRunning(ctx context.Context, embedClient llm.LLMClient) (store.VectorStore, error) {
	if q.st != nil {
		return q.st, nil
	}

	st, err := q.tryConnect(ctx)
	if err == nil {
		q.st = st
		q.ensureCollections(ctx, st, embedClient)
		return st, nil
	}

	if !q.cfg.AutoStart {
		return nil, fmt.Errorf("qdrant not reachable at %s:%d — start it manually or set rag.qdrant.auto_start=true: %w",
			q.cfg.Host, q.cfg.Port, err)
	}

	if err := q.startContainer(ctx); err != nil {
		return nil, fmt.Errorf("failed to auto-start qdrant: %w", err)
	}

	st, err = q.waitForReady(ctx)
	if err != nil {
		return nil, fmt.Errorf("qdrant did not become ready after auto-start: %w", err)
	}

	q.st = st
	q.ownContainer = true

	q.ensureCollections(ctx, st, embedClient)

	return st, nil
}

func (q *Qdrant) ensureCollections(ctx context.Context, st store.VectorStore, embedClient llm.LLMClient) {
	dim := uint64(q.cfg.VectorDim)
	if err := st.InitCollections(ctx, dim); err != nil {
		return
	}
}

func (q *Qdrant) Stop(ctx context.Context) error {
	if !q.ownContainer || q.cfg.ContainerName == "" {
		return nil
	}

	stop := exec.CommandContext(ctx, "docker", "stop", q.cfg.ContainerName)
	if out, err := stop.CombinedOutput(); err != nil {
		return fmt.Errorf("docker stop %s: %w\n%s", q.cfg.ContainerName, err, out)
	}

	rm := exec.CommandContext(ctx, "docker", "rm", q.cfg.ContainerName)
	if out, err := rm.CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm %s: %w\n%s", q.cfg.ContainerName, err, out)
	}

	q.ownContainer = false
	return nil
}

func (q *Qdrant) tryConnect(ctx context.Context) (store.VectorStore, error) {
	st, err := store.New(q.cfg.Host, q.cfg.Port)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if _, err := st.ListCollections(pingCtx); err != nil {
		st.Close()
		return nil, err
	}

	return st, nil
}

func (q *Qdrant) startContainer(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found in PATH — install Docker Desktop or start Qdrant manually on %s:%d", q.cfg.Host, q.cfg.Port)
	}

	exists := exec.CommandContext(ctx, "docker", "inspect", q.cfg.ContainerName)
	if exists.Run() == nil {
		running := exec.CommandContext(ctx, "docker", "container", "inspect", "-f", "{{.State.Running}}", q.cfg.ContainerName)
		if out, err := running.Output(); err == nil && string(out) == "true\n" {
			return nil
		}

		cleanup := exec.CommandContext(ctx, "docker", "rm", "-f", q.cfg.ContainerName)
		if out, err := cleanup.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to remove stale container %s: %w\n%s", q.cfg.ContainerName, err, out)
		}
	}

	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", q.cfg.ContainerName,
		"-p", fmt.Sprintf("%d:6334", q.cfg.Port),
		"qdrant/qdrant",
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker run qdrant/qdrant: %w\n%s", err, out)
	}

	return nil
}

func (q *Qdrant) waitForReady(ctx context.Context) (store.VectorStore, error) {
	deadline := time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		st, err := q.tryConnect(ctx)
		if err == nil {
			return st, nil
		}

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("timeout waiting for Qdrant (%s)", pollTimeout)
}
