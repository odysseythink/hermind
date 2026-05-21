package sqlite

import (
	"context"
	"fmt"
	"testing"

	"github.com/odysseythink/hermind/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfile_GetNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.GetProfile(context.Background(), "nobody")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestProfile_SaveAndGet(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	version, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "alice",
		Adds: []storage.ProfileSection{
			{Kind: "explicit", Key: "diet.restrictions", Value: "peanuts", Evidence: "said so", SourceTurns: []int64{1}, Confidence: 0.9},
			{Kind: "implicit", Key: "style.communication", Value: "terse", Evidence: "short replies", SourceTurns: []int64{2, 3}, Confidence: 0.7},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), version)

	p, err := store.GetProfile(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, p.Sections, 2)
	assert.Equal(t, "explicit", p.Sections[0].Kind)
	assert.Equal(t, "diet.restrictions", p.Sections[0].Key)
	assert.Equal(t, "peanuts", p.Sections[0].Value)
	assert.Equal(t, "implicit", p.Sections[1].Kind)
	assert.Equal(t, "style.communication", p.Sections[1].Key)
}

func TestProfile_Update(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "bob",
		Adds: []storage.ProfileSection{
			{Kind: "explicit", Key: "work.role", Value: "engineer", Evidence: "initial", Confidence: 0.8},
		},
	})
	require.NoError(t, err)

	version, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "bob",
		Updates: []storage.ProfileSection{
			{Kind: "explicit", Key: "work.role", Value: "senior engineer", Evidence: "promotion", Confidence: 0.95},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), version)

	p, err := store.GetProfile(ctx, "bob")
	require.NoError(t, err)
	require.Len(t, p.Sections, 1)
	assert.Equal(t, "senior engineer", p.Sections[0].Value)
	assert.Equal(t, "promotion", p.Sections[0].Evidence)
	assert.Equal(t, 0.95, p.Sections[0].Confidence)
}

func TestProfile_Delete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "carol",
		Adds: []storage.ProfileSection{
			{Kind: "explicit", Key: "a", Value: "1"},
			{Kind: "explicit", Key: "b", Value: "2"},
		},
	})
	require.NoError(t, err)

	_, err = store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "carol",
		Deletes: []storage.ProfileSectionRef{
			{UserID: "carol", Kind: "explicit", Key: "a"},
		},
	})
	require.NoError(t, err)

	p, err := store.GetProfile(ctx, "carol")
	require.NoError(t, err)
	require.Len(t, p.Sections, 1)
	assert.Equal(t, "b", p.Sections[0].Key)
}

func TestProfile_MixedDelta(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Seed two sections.
	_, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "dave",
		Adds: []storage.ProfileSection{
			{Kind: "explicit", Key: "x", Value: "old-x"},
			{Kind: "explicit", Key: "y", Value: "old-y"},
			{Kind: "explicit", Key: "z", Value: "old-z"},
		},
	})
	require.NoError(t, err)

	// Add w, update y, delete z.
	version, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
		UserID: "dave",
		Adds: []storage.ProfileSection{
			{Kind: "explicit", Key: "w", Value: "new-w"},
		},
		Updates: []storage.ProfileSection{
			{Kind: "explicit", Key: "y", Value: "updated-y"},
		},
		Deletes: []storage.ProfileSectionRef{
			{UserID: "dave", Kind: "explicit", Key: "z"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), version)

	p, err := store.GetProfile(ctx, "dave")
	require.NoError(t, err)
	require.Len(t, p.Sections, 3)
	keys := []string{p.Sections[0].Key, p.Sections[1].Key, p.Sections[2].Key}
	assert.ElementsMatch(t, []string{"w", "x", "y"}, keys)
}

func TestProfile_ConcurrentSaves(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	done := make(chan int64, 2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			v, err := store.SaveProfileDelta(ctx, &storage.ProfileDelta{
				UserID: "eve",
				Adds: []storage.ProfileSection{
					{Kind: "explicit", Key: fmt.Sprintf("k%d", idx), Value: fmt.Sprintf("v%d", idx)},
				},
			})
			if err != nil {
				done <- -1
				return
			}
			done <- v
		}(i)
	}

	var versions []int64
	for i := 0; i < 2; i++ {
		v := <-done
		require.NotEqual(t, int64(-1), v, "concurrent save failed")
		versions = append(versions, v)
	}

	// Both versions should be distinct and sequential.
	assert.NotEqual(t, versions[0], versions[1])
	assert.ElementsMatch(t, []int64{1, 2}, versions)

	p, err := store.GetProfile(ctx, "eve")
	require.NoError(t, err)
	require.Len(t, p.Sections, 2)
}
