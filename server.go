package raft

type rpcResponse[T any] struct {
	resp  T
	error error
}
