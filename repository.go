package docxtidy

import "context"

type Repository interface {
	Save(ctx context.Context, docID string, state State) error
	Load(ctx context.Context, docID string) (State, error)
	Delete(ctx context.Context, docID string) error
}
