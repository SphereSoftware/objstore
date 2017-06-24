package journal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var noID ID = ""

func TestBtreeDiffBtree(t *testing.T) {
	assert := assert.New(t)

	j1 := MakeJournal(noID, []*Meta{
		{ID: "000"}, {ID: "001"}, {ID: "002"}, {ID: "003"}, {ID: "005"},
	})
	j2 := MakeJournal(noID, []*Meta{
		{ID: "000"}, {ID: "002"}, {ID: "003"}, {ID: "004"}, {ID: "005"},
	})

	added, deleted := j1.Diff(j2)
	assert.Equal([]*Meta{{ID: "004"}}, added)
	assert.Equal([]*Meta{{ID: "001"}}, deleted)

	added, deleted = j1.Diff(j1)
	assert.Empty(added)
	assert.Empty(deleted)
}
