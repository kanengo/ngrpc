package selector

import (
	"context"
)

type Selector interface {
	Select(ctx context.Context)
}
