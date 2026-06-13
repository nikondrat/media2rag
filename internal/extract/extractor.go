package extract

import (
	"context"
	"errors"
)

type Extractor interface {
	Detect(path string) bool
	Extract(ctx context.Context, path string) (string, error)
	ContentType() string
}

const (
	ContentTypeTranscript = "transcript"
	ContentTypeBook       = "book"
	ContentTypeClean      = "clean"
)

type Registry struct {
	extractors []Extractor
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(extractor Extractor) {
	r.extractors = append(r.extractors, extractor)
}

func (r *Registry) Find(path string) (Extractor, error) {
	for _, e := range r.extractors {
		if e.Detect(path) {
			return e, nil
		}
	}
	return nil, ErrNoExtractor
}

var ErrNoExtractor = errors.New("no extractor found for the given source")
