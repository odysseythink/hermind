package bedrock

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/odysseythink/hermind/provider"
)

// mapAWSError translates a bedrockruntime modeled error into the shared
// provider.Error taxonomy. Unknown errors become ErrUnknown so the
// caller can still display them without treating them as retryable.
func mapAWSError(err error) error {
	if err == nil {
		return nil
	}
	// If it is already a provider.Error, pass through unchanged.
	var pErr *provider.Error
	if errors.As(err, &pErr) {
		return err
	}

	kind := provider.ErrUnknown

	var (
		throttling       *types.ThrottlingException
		accessDenied     *types.AccessDeniedException
		validation       *types.ValidationException
		serviceQuota     *types.ServiceQuotaExceededException
		internalServer   *types.InternalServerException
		modelTimeout     *types.ModelTimeoutException
		resourceNotFound *types.ResourceNotFoundException
		modelNotReady    *types.ModelNotReadyException
		modelError       *types.ModelErrorException
		streamError      *types.ModelStreamErrorException
		unavailable      *types.ServiceUnavailableException
	)

	switch {
	case errors.As(err, &throttling), errors.As(err, &serviceQuota):
		kind = provider.ErrRateLimit
	case errors.As(err, &accessDenied):
		kind = provider.ErrAuth
	case errors.As(err, &validation), errors.As(err, &resourceNotFound):
		kind = provider.ErrInvalidRequest
	case errors.As(err, &internalServer), errors.As(err, &modelError),
		errors.As(err, &streamError), errors.As(err, &unavailable),
		errors.As(err, &modelNotReady):
		kind = provider.ErrServerError
	case errors.As(err, &modelTimeout):
		kind = provider.ErrTimeout
	}

	return &provider.Error{
		Kind:     kind,
		Provider: "bedrock",
		Message:  fmt.Sprintf("aws: %v", err),
		Cause:    err,
	}
}
