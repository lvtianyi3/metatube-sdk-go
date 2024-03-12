package javbus

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJavBus_GetMovieInfoByID(t *testing.T) {
	provider := New()
	for _, item := range []string{
		"SMBD-77",
		"SSNI-776",
		"ABP-331",
		"CEMD-232",
	} {
		info, err := provider.GetMovieInfoByID(item)
		data, _ := json.MarshalIndent(info, "", "\t")
		assert.True(t, assert.NoError(t, err) && assert.True(t, info.Valid()))
		t.Logf("%s", data)
	}
}

func TestJavBus_SearchMovie(t *testing.T) {
	provider := New()
	for _, item := range []string{
		"SSIS-033",
		"MIDV-005",
	} {
		results, err := provider.SearchMovie(provider.NormalizeMovieKeyword(item))
		data, _ := json.MarshalIndent(results, "", "\t")
		if assert.NoError(t, err) {
			for _, result := range results {
				assert.True(t, result.Valid())
			}
		}
		t.Logf("%s", data)
	}
}

func TestJavBus_GetMovieInfoByID2(t *testing.T) {
	provider := New()

	// 打开CSV文件以供写入
	file, err := os.Create("magnet.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	// 创建CSV写入器
	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, item := range []string{} {
		results, err := provider.GetMagnetInfoByID(provider.NormalizeMovieKeyword(item))
		for _, result := range results {

			// 将结构体写入CSV文件
			err = writer.Write([]string{result.ID, result.Title, result.Magnet, result.Size})
			if err != nil {
				panic(err)
			}
		}
	}

	println("结构体已成功写入CSV文件.")
}
