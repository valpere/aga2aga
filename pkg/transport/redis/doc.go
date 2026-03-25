// Package redistransport implements transport.Transport for Redis Streams.
// Publish uses XADD; Subscribe uses XGROUP CREATE + XREADGROUP consumer groups;
// Ack uses XACK. Close cancels the internal context, stopping all subscribe loops.
//
// The package name is redistransport (not redis) to avoid collision with the
// github.com/redis/go-redis/v9 package imported as goredis.
package redistransport
