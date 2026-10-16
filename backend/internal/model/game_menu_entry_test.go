package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameMenuEntries_ValueScan(t *testing.T) {
	v, err := GameMenuEntries(nil).Value()
	require.NoError(t, err)
	assert.Nil(t, v, "nil lưu NULL")
	v, err = GameMenuEntries{}.Value()
	require.NoError(t, err)
	assert.Nil(t, v, "rỗng cũng lưu NULL")

	in := GameMenuEntries{{DisplayName: "Word", Launcher: "program/swriter.exe"}, {DisplayName: "Excel", Launcher: "program/scalc.exe", Icon: "x.ico"}}
	v, err = in.Value()
	require.NoError(t, err)
	var out GameMenuEntries
	require.NoError(t, out.Scan([]byte(v.(string))))
	assert.Equal(t, in, out)

	for _, src := range []interface{}{nil, "", "null", []byte("null")} {
		out = GameMenuEntries{{DisplayName: "cũ"}}
		require.NoError(t, out.Scan(src))
		assert.Nil(t, out, "%v", src)
	}
	assert.Error(t, out.Scan(42))
	assert.Error(t, out.Scan("{"))
}
