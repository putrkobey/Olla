package logger

import "sync"

// ThrottleSet collects the throttles one owner created so it can settle them
// together at shutdown. Each Throttle only writes a summary from inside Allow
// (a later rejection through that same throttle) or Flush, so without a Flush
// the tail of a flood that stops just before shutdown is never reported.
//
