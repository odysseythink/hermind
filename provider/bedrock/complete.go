package bedrock

import (
	"context"
	"fmt"

	"github.com/odysseythink/hermind/provider"
)

// Complete sends a non-streaming Converse request to Bedrock.
func (b *Bedrock) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	in := buildConverseInput(req)
	out, err := b.client.Converse(ctx, in)
	if err != nil {
		return nil, mapAWSError(err)
	}
	if out == nil || out.Output == nil {
		return nil, &provider.Error{
			Kind:     provider.ErrServerError,
			Provider: "bedrock",
			Message:  fmt.Sprintf("empty Converse response for model %q", req.Model),
		}
	}
	return convertConverseOutput(out, req.Model), nil
}
