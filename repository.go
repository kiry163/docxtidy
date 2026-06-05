package docxtidy

import "context"

type Repository interface {
	Save(ctx context.Context, docID string, state DocumentState) error
	Load(ctx context.Context, docID string) (DocumentState, error)
	Delete(ctx context.Context, docID string) error
}
