// Package application is the application block of the reservations bounded
// context: the consumer side of FND-04 §6.3, walking its seven steps through
// the inbox instead of an inbound command (KRN-07), and the synchronous
// Reserve and Cancel of §3.2.
package application
