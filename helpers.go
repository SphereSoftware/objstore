package objstore

import "sphere.software/objstore/cluster"

// PUMP CODE — a circular buffer
// Copyright 2014 The Go Authors
//
// pumpEventAnnounces returns a channel src such that sending on src will eventually send on
// dst, in order, but that src will always be ready to send/receive soon, even
// if dst currently isn't. It is effectively an infinitely buffered channel.
//
// In particular, goroutine A sending on src will not deadlock even if goroutine
// B that's responsible for receiving on dst is currently blocked trying to
// send to A on a separate channel.
//
// Send a EventStopAnnounce event on the src channel to close the dst channel after all queued
// events are sent on dst. After that, other goroutines can still send to src,
// so that such sends won't block forever, but such events will be ignored.
func pumpEventAnnounces(dst chan *EventAnnounce) (src chan *EventAnnounce) {
	src = make(chan *EventAnnounce)
	go func() {
		// initialSize is the initial size of the circular buffer. It must be a
		// power of 2.
		const initialSize = 16
		i, j, buf, mask := 0, 0, make([]*EventAnnounce, initialSize), initialSize-1

		maybeSrc := src
		for {
			maybeDst := dst
			if i == j {
				maybeDst = nil
			}
			if maybeDst == nil && maybeSrc == nil {
				break
			}

			select {
			case maybeDst <- buf[i&mask]:
				buf[i&mask] = nil
				i++

			case e := <-maybeSrc:
				if e.Type == cluster.EventStopAnnounce {
					maybeSrc = nil
					continue
				}

				// Allocate a bigger buffer if necessary.
				if i+len(buf) == j {
					b := make([]*EventAnnounce, 2*len(buf))
					n := copy(b, buf[j&mask:])
					copy(b[n:], buf[:j&mask])
					i, j = 0, len(buf)
					buf, mask = b, len(b)-1
				}

				buf[j&mask] = e
				j++
			}
		}

		close(dst)
		// Block forever.
		for range src {
		}
	}()
	return src
}
