package ffmpeg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitCommand(t *testing.T) {
	cmd := `-y -i ${INPUT_MEDIA} -vf "scale=1280:-1" -c:v libx264 out.mp4`
	expected := []string{"-y", "-i", "${INPUT_MEDIA}", "-vf", "scale=1280:-1", "-c:v", "libx264", "out.mp4"}

	args, err := SplitCommand(cmd)
	assert.NoError(t, err)
	assert.Equal(t, expected, args)
}

func TestSanitizeAndValidateArgs(t *testing.T) {
	t.Run("Valid command", func(t *testing.T) {
		args, _ := SplitCommand(`-i ${INPUT_MEDIA} -c:v libx264`)
		err := SanitizeAndValidateArgs(args)
		assert.NoError(t, err)
	})

	t.Run("Missing input placeholder", func(t *testing.T) {
		args, _ := SplitCommand(`-i somefile.mp4 -c:v libx264`)
		err := SanitizeAndValidateArgs(args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must include the input placeholder")
	})

	t.Run("Disallowed character (semicolon)", func(t *testing.T) {
		args, _ := SplitCommand(`-i ${INPUT_MEDIA}; ls`)
		err := SanitizeAndValidateArgs(args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "disallowed character found in argument: ${INPUT_MEDIA};")
	})

	t.Run("Disallowed character (dollar)", func(t *testing.T) {
		args, _ := SplitCommand(`-i ${INPUT_MEDIA} -vf "crop=$(($RANDOM))"`)
		err := SanitizeAndValidateArgs(args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "disallowed character found in argument: crop=$(($RANDOM))")
	})
}