# ratelimiter_grpc
RateLimiter helps ratelimit the grpc request based on the config and value to check defined by the user.
For eg. Possible brute force can be detected and request will not be passed through the service
## Installation
    go get github.com/rohitsubedi/ratelimiter_grpc@v1.0.0
## Testing
    make test
## Avalilable methods
### RateLimit using system memory
```golang
limiter := ratelimiter_grpc.NewRateLimiterUsingMemory(cacheCleaningInterval)
limiter.UnaryRateLimit(conf) // Ratelimit normal grpc method
limiter.StreamRateLimit(conf) // Ratelimit stream grpc method

//when conf is passed nil, then the rate limit will be skipped and works as there is no ratelimiting
//conf should follow ConfigReaderInterface which has 1 method
    //IsMethodRateLimited(ctx context.Context, fullMethodName string) (bool, *RateLimitingInfo)
//It should return if the rate limit is enabled for given path with the info
    //RateLimitingInfo struct {
        //LimitKeyValue                string // The value that will be rate limited. eg. ip address
        //TimeFrameDurationToCheck     time.Duration // Time frame for rate limiting. eg. In 1 hour if only 3 request is allowed, it should be 1 hour and next one should be 3
        //MaxRequestAllowedInTimeFrame int // Max request allowed in the time frame
        //LimitExceedError             error // error in case of rate limit exceeded
    //}
```
### RateLimit using redis
```golang
limiter := ratelimiter_grpc.NewRateLimiterUsingRedis(host, password)
limiter.UnaryRateLimit(conf) // Ratelimit normal grpc method
limiter.StreamRateLimit(conf) // Ratelimit stream grpc method
//Every parameter is same as above
```

```html
//In both of the above cases, the error and infos are logged with default go logger. You can either disable the logs
//or initialize your own log the follows the following interface
type LeveledLogger interface {
    Error(args ...interface{})
    Info(args ...interface{})
}
```

## Example

```golang
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/rohitsubedi/ratelimiter_grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type config struct{}

func (c *config) IsMethodRateLimited(ctx context.Context, fullMethodName string) (bool, *ratelimiter_grpc.RateLimitingInfo) {
	return true, &ratelimiter_grpc.RateLimitingInfo{
		LimitKeyValue:                "ip_address",
		TimeFrameDurationToCheck:     10 * time.Second,
		MaxRequestAllowedInTimeFrame: 10,
		LimitExceedError:             fmt.Errorf("rate limit exceeded"),
	}
}
func main() {
	port := ":7689"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal(err)
	}

	memLimiter := ratelimiter_grpc.NewRateLimiterUsingMemory(10 * time.Minute)
	// redisLimiter, err := ratelimiter_grpc.NewRateLimiterUsingMemory(host, password)
	// You can chain multiple interceptor as needed. For ex. You can add Authorization interceptor etc
	// memLimiter.SetLogger(nil) can be LeveledLogger interface
	conf := new(config)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(memLimiter.UnaryRateLimit(conf)), grpc.ChainStreamInterceptor(memLimiter.StreamRateLimit(conf)))
	handler := &someHandler{}
	pb.RegisterSomeService(srv, &handler)
	reflection.Register(srv)

	log.Printf("Starting server on port: %s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

```
