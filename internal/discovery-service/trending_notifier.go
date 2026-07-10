package discovery

import (
	"context"
	"fmt"
	"log"
	"time"

	trackdb "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
)

// TrendingNotifier periodically checks for newly-approved tracks that haven't
// been announced yet and broadcasts a push notification for each, reusing the
// same SendPushNotification RPC the manual admin "send notification" button
// already calls — so both paths land through one code path in auth-service.
type TrendingNotifier struct {
	repo       trackdb.Querier
	authClient authpb.AuthServiceClient
	interval   time.Duration
}

func NewTrendingNotifier(repo trackdb.Querier, authClient authpb.AuthServiceClient, interval time.Duration) *TrendingNotifier {
	return &TrendingNotifier{repo: repo, authClient: authClient, interval: interval}
}

// Start blocks — run it in its own goroutine (`go notifier.Start(ctx)`).
func (n *TrendingNotifier) Start(ctx context.Context) {
	log.Printf("📣 Trending notifier started — checking every %s", n.interval)

	n.checkAndNotify(ctx)

	ticker := time.NewTicker(n.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n.checkAndNotify(ctx)
		case <-ctx.Done():
			log.Println("🛑 Trending notifier stopped")
			return
		}
	}
}

// checkAndNotify bundles everything found in a single polling cycle into ONE
// push notification, never one-per-track — a discovery cycle can insert
// dozens of tracks at once (auto-approved), and firing a separate push per
// track would spam every user with a burst of notifications.
func (n *TrendingNotifier) checkAndNotify(ctx context.Context) {
	tracks, err := n.repo.GetApprovedUnnotifiedTracks(ctx)
	if err != nil {
		log.Printf("⚠️ Trending notifier: failed to fetch unnotified tracks: %v", err)
		return
	}
	if len(tracks) == 0 {
		return
	}

	log.Printf("📣 Trending notifier: %d new track(s) to announce", len(tracks))

	title, body, dataID := buildTrendingNotification(tracks)

	_, err = n.authClient.SendPushNotification(ctx, &authpb.PushNotificationRequest{
		Title:        title,
		Body:         body,
		TargetUserId: "", // empty = broadcast to all registered devices
		DataType:     "trending",
		DataId:       dataID,
	})
	if err != nil {
		log.Printf("⚠️ Failed to broadcast trending push for %d track(s): %v", len(tracks), err)
		return // leave all fcm_notified=false so the whole batch retries next tick
	}

	ids := make([]string, len(tracks))
	for i, track := range tracks {
		ids[i] = track.ID
	}
	if err := n.repo.MarkTracksFCMNotified(ctx, ids); err != nil {
		log.Printf("⚠️ Failed to mark %d track(s) as notified: %v", len(ids), err)
	}

	log.Printf("✅ Announced %d new track(s) in a single push", len(tracks))
}

// buildTrendingNotification returns a specific "Artist — Title" message for a
// single new track, or a summary ("N new tracks dropped") when a batch landed
// together — so users get one push either way, never a flood.
func buildTrendingNotification(tracks []trackdb.Track) (title, body, dataID string) {
	if len(tracks) == 1 {
		t := tracks[0]
		return "New Trending Phonk!", fmt.Sprintf("%s — %s just dropped", t.ArtistName, t.Title), t.ID
	}
	return "New Phonk Drops!", fmt.Sprintf("%d new tracks just dropped — check them out!", len(tracks)), "trending_batch"
}
