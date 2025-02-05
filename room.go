package main

type Room struct {
	users  map[*UserConnection]bool
	join   chan *UserConnection
	leave  chan *UserConnection
	foward chan []byte
}
