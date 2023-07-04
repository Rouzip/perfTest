package rawcollector

import "os"

type goRawCollector struct {
	fd *os.File
}
