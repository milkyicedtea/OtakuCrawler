package scrapers

import (
	"context"
	"golang.org/x/time/rate"
	"io"
)

type TokenBucketRateLimitedReader struct {
	reader  io.Reader
	limiter *rate.Limiter
}

func NewTokenBucketRateLimitedReader(reader io.Reader, maxBytesPerSecond int) *TokenBucketRateLimitedReader {
	burstSize := maxBytesPerSecond / 10 // 100ms of burst data
	if burstSize > 16*1024 {            // Cap at 16KB for stable rates
		burstSize = 16 * 1024
	}

	limiter := rate.NewLimiter(rate.Limit(maxBytesPerSecond), burstSize)

	return &TokenBucketRateLimitedReader{
		reader:  reader,
		limiter: limiter,
	}
}

func (r *TokenBucketRateLimitedReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil {
		return n, err
	}

	// Wait for tokens for the bytes we just read
	if n > 0 {
		err := r.limiter.WaitN(context.Background(), n)
		if err != nil {
			return n, err
		}
	}

	return n, nil
}

func (r *TokenBucketRateLimitedReader) Close() error {
	if closer, ok := r.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
