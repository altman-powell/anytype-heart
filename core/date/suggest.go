package date

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/anyproto/go-naturaldate/v2"
	"github.com/araddon/dateparse"

	"github.com/anyproto/anytype-heart/core/block/source"
	"github.com/anyproto/anytype-heart/core/domain"
	"github.com/anyproto/anytype-heart/pb"
	"github.com/anyproto/anytype-heart/pkg/lib/bundle"
	"github.com/anyproto/anytype-heart/pkg/lib/database"
	"github.com/anyproto/anytype-heart/pkg/lib/localstore/objectstore"
	"github.com/anyproto/anytype-heart/space"
	"github.com/anyproto/anytype-heart/util/dateutil"
	"github.com/anyproto/anytype-heart/util/pbtypes"
)

func EnrichRecordsWithDateSuggestions(
	ctx context.Context,
	records []database.Record,
	req *pb.RpcObjectSearchRequest,
	store objectstore.ObjectStore,
	spaceService space.Service,
) ([]database.Record, error) {
	ids := suggestDateObjectIds(req)
	if len(ids) == 0 {
		return records, nil
	}

	spc, err := spaceService.Get(ctx, req.SpaceId)
	if err != nil {
		return nil, fmt.Errorf("get space: %w", err)
	}

	for _, id := range ids {
		if recordsHasId(records, id) {
			continue
		}

		rec, err := makeSuggestedDateRecord(ctx, spc, id)
		if err != nil {
			return nil, fmt.Errorf("make date record: %w", err)
		}

		f, _ := database.MakeFilters(req.Filters, store.SpaceIndex(req.SpaceId)) //nolint:errcheck
		if f.FilterObject(rec.Details) {
			records = append([]database.Record{rec}, records...)
		}
	}

	return records, nil
}

// suggestDateObjectIds suggests date object ids based on two fields:
// - fullText - if naturalDate successfully parses text into date, resulting date object id is returned
// - filter with key id
func suggestDateObjectIds(req *pb.RpcObjectSearchRequest) []string {
	dt := suggestDateForSearch(time.Now(), req.FullText)
	if !dt.IsZero() {
		// TODO: GO-4097 Uncomment it when we will be able to support dates with seconds precision
		// isDay := dt.Hour() == 0 && dt.Minute() == 0 && dt.Second() == 0
		isDay := true
		return []string{dateutil.NewDateObject(dt, !isDay).Id()}
	}

	for _, filter := range req.Filters {
		if filter.RelationKey == bundle.RelationKeyId.String() {
			list := pbtypes.GetStringListValue(filter.Value)
			var dateObjectIds []string
			for _, id := range list {
				if _, err := dateutil.BuildDateObjectFromId(id); err == nil {
					dateObjectIds = append(dateObjectIds, id)
				}
			}
			return dateObjectIds
		}
	}

	return nil
}

func suggestDateForSearch(now time.Time, raw string) time.Time {
	// a hack to show calendar in case date is typed
	if raw == "date" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	suggesters := []func() time.Time{
		func() time.Time {
			var exprType naturaldate.ExprType
			t, exprType, err := naturaldate.Parse(raw, now)
			if err != nil {
				return time.Time{}
			}
			if exprType == naturaldate.ExprTypeInvalid {
				return time.Time{}
			}

			// naturaldate parses numbers without qualifiers (m,s) as hours in 24 hours clock format. It leads to weird behavior
			// when inputs like "123" represented as "current time + 123 hours"
			if (exprType & naturaldate.ExprTypeClock24Hour) != 0 {
				t = time.Time{}
			}
			return t
		},
		func() time.Time {
			// Don't use plain numbers, because they will be represented as years
			if _, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
				return time.Time{}
			}
			// todo: use system locale to get preferred date format
			t, err := dateparse.ParseIn(raw, now.Location(), dateparse.PreferMonthFirst(false))
			if err != nil {
				return time.Time{}
			}
			return t
		},
	}

	var t time.Time
	for _, s := range suggesters {
		if t = s(); !t.IsZero() {
			break
		}
	}
	if t.IsZero() {
		return t
	}

	// Sanitize date

	// Date without year
	if t.Year() == 0 {
		_, month, day := t.Date()
		h, m, s := t.Clock()
		t = time.Date(now.Year(), month, day, h, m, s, 0, t.Location())
	}

	return t
}

func recordsHasId(records []database.Record, id string) bool {
	for _, r := range records {
		if r.Details == nil || r.Details.Fields == nil {
			continue
		}
		if v, ok := r.Details.Fields[bundle.RelationKeyId.String()]; ok {
			if v.GetStringValue() == id {
				return true
			}
		}

	}
	return false
}

func makeSuggestedDateRecord(ctx context.Context, spc source.Space, id string) (database.Record, error) {
	typeId, err := spc.GetTypeIdByKey(ctx, bundle.TypeKeyDate)
	if err != nil {
		return database.Record{}, fmt.Errorf("failed to find Date type to build Date object: %w", err)
	}

	dateSource := source.NewDate(source.DateSourceParams{
		Id: domain.FullID{
			ObjectID: id,
			SpaceID:  spc.Id(),
		},
		DateObjectTypeId: typeId,
	})

	v, ok := dateSource.(source.SourceIdEndodedDetails)
	if !ok {
		return database.Record{}, fmt.Errorf("source does not implement DetailsFromId")
	}

	details, err := v.DetailsFromId()
	if err != nil {
		return database.Record{}, err
	}

	return database.Record{
		Details: details,
	}, nil
}