package export

import (
	"archive/zip"
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/anyproto/any-sync/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/anyproto/anytype-heart/core/block/cache/mock_cache"
	"github.com/anyproto/anytype-heart/core/block/editor/smartblock/smarttest"
	"github.com/anyproto/anytype-heart/core/block/editor/state"
	"github.com/anyproto/anytype-heart/core/block/process"
	"github.com/anyproto/anytype-heart/core/block/simple"
	"github.com/anyproto/anytype-heart/core/converter/pbjson"
	"github.com/anyproto/anytype-heart/core/domain"
	"github.com/anyproto/anytype-heart/core/event/mock_event"
	"github.com/anyproto/anytype-heart/core/notifications/mock_notifications"
	"github.com/anyproto/anytype-heart/pb"
	"github.com/anyproto/anytype-heart/pkg/lib/bundle"
	"github.com/anyproto/anytype-heart/pkg/lib/core/smartblock"
	"github.com/anyproto/anytype-heart/pkg/lib/localstore/addr"
	"github.com/anyproto/anytype-heart/pkg/lib/localstore/objectstore"
	"github.com/anyproto/anytype-heart/pkg/lib/localstore/objectstore/spaceindex"
	"github.com/anyproto/anytype-heart/pkg/lib/pb/model"
	"github.com/anyproto/anytype-heart/space/spacecore/typeprovider/mock_typeprovider"
	"github.com/anyproto/anytype-heart/tests/testutil"
)

func TestFileNamer_Get(t *testing.T) {
	fn := newNamer()
	names := make(map[string]bool)
	nl := []string{
		"files/some_long_name_12345678901234567890.jpg",
		"files/some_long_name_12345678901234567890.jpg",
		"some_long_name_12345678901234567890.jpg",
		"one.png",
		"two.png",
		"two.png",
		"сделай норм!.pdf",
		"some very long name maybe note or just unreal long title.md",
		"some very long name maybe note or just unreal long title.md",
	}
	for i, v := range nl {
		nm := fn.Get(filepath.Dir(v), fmt.Sprint(i), filepath.Base(v), filepath.Ext(v))
		t.Log(nm)
		names[nm] = true
		assert.NotEmpty(t, nm, v)
	}
	assert.Equal(t, len(names), len(nl))
}

const spaceId = "space1"

func TestExport_Export(t *testing.T) {
	t.Run("export success", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		objectTypeId := "customObjectType"
		objectTypeUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeObjectType, objectTypeId)
		assert.Nil(t, err)

		objectID := "id"
		storeFixture.AddObjects(t, spaceId, []spaceindex.TestObject{
			{
				bundle.RelationKeyId:      domain.String(objectID),
				bundle.RelationKeyType:    domain.String(objectTypeId),
				bundle.RelationKeySpaceId: domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:                   domain.String(objectTypeId),
				bundle.RelationKeyUniqueKey:            domain.String(objectTypeUniqueKey.Marshal()),
				bundle.RelationKeyLayout:               domain.Int64(int64(model.ObjectType_objectType)),
				bundle.RelationKeyRecommendedRelations: domain.StringList([]string{addr.MissingObject}),
				bundle.RelationKeySpaceId:              domain.String(spaceId),
			},
		})

		objectGetter := mock_cache.NewMockObjectGetter(t)

		smartBlockTest := smarttest.New(objectID)
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(objectID),
			bundle.RelationKeyType: domain.String(objectTypeId),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New(objectTypeId)
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(objectTypeId),
			bundle.RelationKeyType: domain.String(objectTypeId),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc
		objectType.SetType(smartblock.SmartBlockTypeObjectType)
		objectGetter.EXPECT().GetObject(context.Background(), objectID).Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), objectTypeId).Return(objectType, nil)

		a := &app.App{}
		mockSender := mock_event.NewMockSender(t)
		mockSender.EXPECT().Broadcast(mock.Anything).Return()
		a.Register(testutil.PrepareMock(context.Background(), a, mockSender))
		service := process.New()
		err = service.Init(a)
		assert.Nil(t, err)

		notifications := mock_notifications.NewMockNotifications(t)
		notifications.EXPECT().CreateAndSend(mock.Anything).Return(nil)

		e := &export{
			objectStore:         storeFixture,
			picker:              objectGetter,
			processService:      service,
			notificationService: notifications,
		}

		// when
		path, success, err := e.Export(context.Background(), pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			Path:          t.TempDir(),
			ObjectIds:     []string{objectID},
			Format:        model.Export_Protobuf,
			Zip:           true,
			IncludeNested: true,
			IncludeFiles:  true,
			IsJson:        true,
		})

		// then
		assert.Nil(t, err)
		assert.Equal(t, 2, success)

		reader, err := zip.OpenReader(path)
		assert.Nil(t, err)

		assert.Len(t, reader.File, 2)
		fileNames := make(map[string]bool, 2)
		for _, file := range reader.File {
			fileNames[file.Name] = true
		}

		objectPath := filepath.Join(objectsDirectory, objectID+".pb.json")
		assert.True(t, fileNames[objectPath])
		typePath := filepath.Join(typesDirectory, objectTypeId+".pb.json")
		assert.True(t, fileNames[typePath])
	})
	t.Run("empty import", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		objectID := "id"

		objectGetter := mock_cache.NewMockObjectGetter(t)

		a := &app.App{}
		mockSender := mock_event.NewMockSender(t)
		mockSender.EXPECT().Broadcast(mock.Anything).Return()
		a.Register(testutil.PrepareMock(context.Background(), a, mockSender))
		service := process.New()
		err := service.Init(a)
		assert.Nil(t, err)

		notifications := mock_notifications.NewMockNotifications(t)
		notifications.EXPECT().CreateAndSend(mock.Anything).Return(nil)

		e := &export{
			objectStore:         storeFixture,
			picker:              objectGetter,
			processService:      service,
			notificationService: notifications,
		}

		// when
		path, success, err := e.Export(context.Background(), pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			Path:          t.TempDir(),
			ObjectIds:     []string{objectID},
			Format:        model.Export_Protobuf,
			Zip:           true,
			IncludeNested: true,
			IncludeFiles:  true,
			IsJson:        true,
		})

		// then
		assert.Nil(t, err)
		assert.Equal(t, 0, success)

		reader, err := zip.OpenReader(path)
		assert.Nil(t, err)
		assert.Len(t, reader.File, 0)
	})
	t.Run("import finished with error", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		objectTypeId := "customObjectType"

		objectID := "id"
		storeFixture.AddObjects(t, spaceId, []spaceindex.TestObject{
			{
				bundle.RelationKeyId:      domain.String(objectID),
				bundle.RelationKeyType:    domain.String(objectTypeId),
				bundle.RelationKeySpaceId: domain.String(spaceId),
			},
		})
		objectGetter := mock_cache.NewMockObjectGetter(t)
		objectGetter.EXPECT().GetObject(context.Background(), objectID).Return(nil, fmt.Errorf("error"))

		a := &app.App{}
		mockSender := mock_event.NewMockSender(t)
		mockSender.EXPECT().Broadcast(mock.Anything).Return()
		a.Register(testutil.PrepareMock(context.Background(), a, mockSender))
		service := process.New()
		err := service.Init(a)
		assert.Nil(t, err)

		notifications := mock_notifications.NewMockNotifications(t)
		notifications.EXPECT().CreateAndSend(mock.Anything).Return(nil)

		e := &export{
			objectStore:         storeFixture,
			picker:              objectGetter,
			processService:      service,
			notificationService: notifications,
		}

		// when
		_, success, err := e.Export(context.Background(), pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			Path:          t.TempDir(),
			ObjectIds:     []string{objectID},
			Format:        model.Export_Protobuf,
			Zip:           true,
			IncludeNested: true,
			IncludeFiles:  true,
			IsJson:        true,
		})

		// then
		assert.NotNil(t, err)
		assert.Equal(t, 0, success)
	})
}

func Test_docsForExport(t *testing.T) {
	t.Run("get object with existing links", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:      domain.String("id"),
				bundle.RelationKeyName:    domain.String("name1"),
				bundle.RelationKeySpaceId: domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:      domain.String("id1"),
				bundle.RelationKeyName:    domain.String("name2"),
				bundle.RelationKeySpaceId: domain.String(spaceId),
			},
		})
		err := storeFixture.SpaceIndex(spaceId).UpdateObjectLinks(context.Background(), "id", []string{"id1"})
		assert.Nil(t, err)

		provider := mock_typeprovider.NewMockSmartBlockTypeProvider(t)
		provider.EXPECT().Type(spaceId, "id1").Return(smartblock.SmartBlockTypePage, nil)
		e := &export{
			objectStore: storeFixture,
			sbtProvider: provider,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 2, len(expCtx.docs))
	})
	t.Run("get object with non existing links", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:      domain.String("id"),
				bundle.RelationKeyName:    domain.String("name"),
				bundle.RelationKeySpaceId: domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:        domain.String("id1"),
				bundle.RelationKeyIsDeleted: domain.Bool(true),
				bundle.RelationKeySpaceId:   domain.String(spaceId),
			},
		})
		err := storeFixture.SpaceIndex(spaceId).UpdateObjectLinks(context.Background(), "id", []string{"id1"})
		assert.Nil(t, err)

		provider := mock_typeprovider.NewMockSmartBlockTypeProvider(t)
		provider.EXPECT().Type(spaceId, "id1").Return(smartblock.SmartBlockTypePage, nil)
		e := &export{
			objectStore: storeFixture,
			sbtProvider: provider,
		}
		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 1, len(expCtx.docs))
	})
	t.Run("get object with non existing relation", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		relationKey := domain.RelationKey("key")
		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:            domain.String("id"),
				domain.RelationKey(relationKey): domain.String("value"),
				bundle.RelationKeyType:          domain.String("objectType"),
				bundle.RelationKeySpaceId:       domain.String(spaceId),
			},
		})
		err := storeFixture.SpaceIndex(spaceId).UpdateObjectLinks(context.Background(), "id", []string{"id1"})
		assert.Nil(t, err)

		objectGetter := mock_cache.NewMockObjectGetter(t)
		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("id"),
			relationKey:            domain.String("value"),
			bundle.RelationKeyType: domain.String("objectType"),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    relationKey.String(),
			Format: model.RelationFormat_longtext,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New("objectType")
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("objectType"),
			bundle.RelationKeyType: domain.String("objectType"),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), "objectType").Return(objectType, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:   spaceId,
			ObjectIds: []string{"id"},
			Format:    model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 1, len(expCtx.docs))
	})
	t.Run("get object with existing relation", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		relationKey := domain.RelationKey("key")
		uniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeRelation, relationKey.String())
		assert.Nil(t, err)

		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:            domain.String("id"),
				domain.RelationKey(relationKey): domain.String("value"),
				bundle.RelationKeyType:          domain.String("objectType"),
				bundle.RelationKeySpaceId:       domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:          domain.String(relationKey),
				bundle.RelationKeyRelationKey: domain.String(relationKey),
				bundle.RelationKeyUniqueKey:   domain.String(uniqueKey.Marshal()),
				bundle.RelationKeySpaceId:     domain.String(spaceId),
			},
		})

		err = storeFixture.SpaceIndex(spaceId).UpdateObjectLinks(context.Background(), "id", []string{"id1"})
		assert.Nil(t, err)

		objectGetter := mock_cache.NewMockObjectGetter(t)

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("id"),
			relationKey:            domain.String("value"),
			bundle.RelationKeyType: domain.String("objectType"),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    relationKey.String(),
			Format: model.RelationFormat_longtext,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New("objectType")
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("objectType"),
			bundle.RelationKeyType: domain.String("objectType"),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), "objectType").Return(objectType, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}
		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:   spaceId,
			ObjectIds: []string{"id"},
			Format:    model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 2, len(expCtx.docs))
	})

	t.Run("get relation options - no relation options", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		relationKey := domain.RelationKey("key")
		uniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeRelation, relationKey.String())
		assert.Nil(t, err)

		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:            domain.String("id"),
				domain.RelationKey(relationKey): domain.String("value"),
				bundle.RelationKeyType:          domain.String("objectType"),
				bundle.RelationKeySpaceId:       domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:             domain.String(relationKey),
				bundle.RelationKeyRelationKey:    domain.String(relationKey),
				bundle.RelationKeyUniqueKey:      domain.String(uniqueKey.Marshal()),
				bundle.RelationKeyRelationFormat: domain.Int64(int64(model.RelationFormat_status)),
				bundle.RelationKeySpaceId:        domain.String(spaceId),
			},
		})

		objectGetter := mock_cache.NewMockObjectGetter(t)

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("id"),
			relationKey:            domain.String("value"),
			bundle.RelationKeyType: domain.String("objectType"),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    relationKey.String(),
			Format: model.RelationFormat_tag,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New("objectType")
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("objectType"),
			bundle.RelationKeyType: domain.String("objectType"),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), "objectType").Return(objectType, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
			Format:        model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 2, len(expCtx.docs))
	})
	t.Run("get relation options - 1 relation option", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		relationKey := domain.RelationKey("key")
		optionId := "optionId"
		uniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeRelation, relationKey.String())
		assert.Nil(t, err)
		optionUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeRelationOption, optionId)
		assert.Nil(t, err)

		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:            domain.String("id"),
				domain.RelationKey(relationKey): domain.String(optionId),
				bundle.RelationKeyType:          domain.String("objectType"),
				bundle.RelationKeySpaceId:       domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:             domain.String(relationKey),
				bundle.RelationKeyRelationKey:    domain.String(relationKey),
				bundle.RelationKeyUniqueKey:      domain.String(uniqueKey.Marshal()),
				bundle.RelationKeyRelationFormat: domain.Int64(int64(model.RelationFormat_tag)),
				bundle.RelationKeyLayout:         domain.Int64(int64(model.ObjectType_relation)),
				bundle.RelationKeySpaceId:        domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:          domain.String(optionId),
				bundle.RelationKeyRelationKey: domain.String(relationKey),
				bundle.RelationKeyUniqueKey:   domain.String(optionUniqueKey.Marshal()),
				bundle.RelationKeyLayout:      domain.Int64(int64(model.ObjectType_relationOption)),
				bundle.RelationKeySpaceId:     domain.String(spaceId),
			},
		})

		objectGetter := mock_cache.NewMockObjectGetter(t)

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("id"),
			relationKey:            domain.String("value"),
			bundle.RelationKeyType: domain.String("objectType"),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    relationKey.String(),
			Format: model.RelationFormat_tag,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New("objectType")
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("objectType"),
			bundle.RelationKeyType: domain.String("objectType"),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), "objectType").Return(objectType, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
			Format:        model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 3, len(expCtx.docs))
		var objectIds []string
		for objectId := range expCtx.docs {
			objectIds = append(objectIds, objectId)
		}
		assert.Contains(t, objectIds, optionId)
	})
	t.Run("get derived objects - relation, object type with recommended relations, template with link", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		relationKey := domain.RelationKey("key")
		objectTypeKey := "customObjectType"
		objectTypeUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeObjectType, objectTypeKey)
		assert.Nil(t, err)
		uniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeRelation, relationKey.String())
		assert.Nil(t, err)

		recommendedRelationKey := "recommendedRelationKey"
		recommendedRelationUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeRelation, recommendedRelationKey)
		assert.Nil(t, err)

		templateId := "templateId"
		templateObjectTypeId := "templateObjectTypeId"

		linkedObjectId := "linkedObjectId"
		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:            domain.String("id"),
				domain.RelationKey(relationKey): domain.String("test"),
				bundle.RelationKeyType:          domain.String(objectTypeKey),
				bundle.RelationKeySpaceId:       domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:          domain.String(relationKey),
				bundle.RelationKeyRelationKey: domain.String(relationKey),
				bundle.RelationKeyUniqueKey:   domain.String(uniqueKey.Marshal()),
				bundle.RelationKeyLayout:      domain.Int64(int64(model.ObjectType_relation)),
				bundle.RelationKeySpaceId:     domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:                   domain.String(objectTypeKey),
				bundle.RelationKeyUniqueKey:            domain.String(objectTypeUniqueKey.Marshal()),
				bundle.RelationKeyLayout:               domain.Int64(int64(model.ObjectType_objectType)),
				bundle.RelationKeyRecommendedRelations: domain.StringList([]string{recommendedRelationKey}),
				bundle.RelationKeySpaceId:              domain.String(spaceId),
				bundle.RelationKeyType:                 domain.String(objectTypeKey),
			},
			{
				bundle.RelationKeyId:          domain.String(recommendedRelationKey),
				bundle.RelationKeyRelationKey: domain.String(recommendedRelationKey),
				bundle.RelationKeyUniqueKey:   domain.String(recommendedRelationUniqueKey.Marshal()),
				bundle.RelationKeyLayout:      domain.Int64(int64(model.ObjectType_relation)),
				bundle.RelationKeySpaceId:     domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:               domain.String(templateId),
				bundle.RelationKeyTargetObjectType: domain.String(objectTypeKey),
				bundle.RelationKeySpaceId:          domain.String(spaceId),
				bundle.RelationKeyType:             domain.String(templateObjectTypeId),
			},
			{
				bundle.RelationKeyId:      domain.String(linkedObjectId),
				bundle.RelationKeyType:    domain.String(objectTypeKey),
				bundle.RelationKeySpaceId: domain.String(spaceId),
			},
		})

		err = storeFixture.SpaceIndex(spaceId).UpdateObjectLinks(context.Background(), templateId, []string{linkedObjectId})
		assert.Nil(t, err)

		objectGetter := mock_cache.NewMockObjectGetter(t)

		template := smarttest.New(templateId)
		templateDoc := template.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(templateId),
			bundle.RelationKeyType: domain.String(templateObjectTypeId),
		}))
		templateDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		template.Doc = templateDoc

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("id"),
			relationKey:            domain.String("value"),
			bundle.RelationKeyType: domain.String(objectTypeKey),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    relationKey.String(),
			Format: model.RelationFormat_tag,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New(objectTypeKey)
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(objectTypeKey),
			bundle.RelationKeyType: domain.String(objectTypeKey),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		templateObjectType := smarttest.New(objectTypeKey)
		templateObjectTypeDoc := templateObjectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(templateId),
			bundle.RelationKeyType: domain.String(templateObjectTypeId),
		}))
		templateObjectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		templateObjectType.Doc = templateObjectTypeDoc

		linkedObject := smarttest.New(objectTypeKey)
		linkedObjectDoc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(linkedObjectId),
			bundle.RelationKeyType: domain.String(objectTypeKey),
		}))
		linkedObjectDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		linkedObject.Doc = linkedObjectDoc

		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), templateId).Return(template, nil)
		objectGetter.EXPECT().GetObject(context.Background(), objectTypeKey).Return(objectType, nil)
		objectGetter.EXPECT().GetObject(context.Background(), templateObjectTypeId).Return(templateObjectType, nil)
		objectGetter.EXPECT().GetObject(context.Background(), linkedObjectId).Return(linkedObject, nil)

		provider := mock_typeprovider.NewMockSmartBlockTypeProvider(t)
		provider.EXPECT().Type(spaceId, linkedObjectId).Return(smartblock.SmartBlockTypePage, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
			sbtProvider: provider,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
			Format:        model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 6, len(expCtx.docs))
	})
	t.Run("get derived objects, object type have missing relations - return only object and its type", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		objectTypeId := "customObjectType"
		objectTypeUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeObjectType, objectTypeId)
		assert.Nil(t, err)

		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:      domain.String("id"),
				bundle.RelationKeyType:    domain.String(objectTypeId),
				bundle.RelationKeySpaceId: domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:                   domain.String(objectTypeId),
				bundle.RelationKeyUniqueKey:            domain.String(objectTypeUniqueKey.Marshal()),
				bundle.RelationKeyLayout:               domain.Int64(int64(model.ObjectType_objectType)),
				bundle.RelationKeyRecommendedRelations: domain.StringList([]string{addr.MissingObject}),
				bundle.RelationKeySpaceId:              domain.String(spaceId),
			},
		})

		objectGetter := mock_cache.NewMockObjectGetter(t)

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("id"),
			bundle.RelationKeyType: domain.String(objectTypeId),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New(objectTypeId)
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(objectTypeId),
			bundle.RelationKeyType: domain.String(objectTypeId),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), objectTypeId).Return(objectType, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
			Format:        model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 2, len(expCtx.docs))
	})
	t.Run("objects without links", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		objectTypeId := "objectTypeId"
		objectTypeUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeObjectType, objectTypeId)
		assert.Nil(t, err)

		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:      domain.String("id"),
				bundle.RelationKeyName:    domain.String("name1"),
				bundle.RelationKeySpaceId: domain.String(spaceId),
				bundle.RelationKeyType:    domain.String(objectTypeId),
			},
			{
				bundle.RelationKeyId:      domain.String("id1"),
				bundle.RelationKeyName:    domain.String("name2"),
				bundle.RelationKeySpaceId: domain.String(spaceId),
				bundle.RelationKeyType:    domain.String(objectTypeId),
			},
			{
				bundle.RelationKeyId:        domain.String(objectTypeId),
				bundle.RelationKeyUniqueKey: domain.String(objectTypeUniqueKey.Marshal()),
				bundle.RelationKeyLayout:    domain.Int64(int64(model.ObjectType_objectType)),
				bundle.RelationKeySpaceId:   domain.String(spaceId),
				bundle.RelationKeyType:      domain.String(objectTypeId),
			},
		})

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("id"),
			bundle.RelationKeyType: domain.String(objectTypeId),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New(objectTypeId)
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(objectTypeId),
			bundle.RelationKeyType: domain.String(objectTypeId),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		objectGetter := mock_cache.NewMockObjectGetter(t)
		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), objectTypeId).Return(objectType, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
			Format:        model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 2, len(expCtx.docs))
	})
	t.Run("objects with dataview", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		objectTypeId := "objectTypeId"
		objectTypeUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeObjectType, objectTypeId)
		assert.Nil(t, err)

		relationKey := "key"
		relationKeyUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeRelation, relationKey)
		assert.Nil(t, err)

		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:      domain.String("id"),
				bundle.RelationKeyName:    domain.String("name1"),
				bundle.RelationKeySpaceId: domain.String(spaceId),
				bundle.RelationKeyType:    domain.String(objectTypeId),
				bundle.RelationKeyLayout:  domain.Int64(int64(model.ObjectType_set)),
			},
			{
				bundle.RelationKeyId:        domain.String(objectTypeId),
				bundle.RelationKeyUniqueKey: domain.String(objectTypeUniqueKey.Marshal()),
				bundle.RelationKeyLayout:    domain.Int64(int64(model.ObjectType_objectType)),
				bundle.RelationKeySpaceId:   domain.String(spaceId),
				bundle.RelationKeyType:      domain.String(objectTypeId),
			},
			{
				bundle.RelationKeyId:          domain.String(bundle.RelationKeyTag.String()),
				bundle.RelationKeyName:        domain.String(bundle.RelationKeyTag.String()),
				bundle.RelationKeyRelationKey: domain.String(bundle.RelationKeyTag.String()),
				bundle.RelationKeySpaceId:     domain.String(spaceId),
				bundle.RelationKeyLayout:      domain.Int64(int64(model.ObjectType_relation)),
				bundle.RelationKeyUniqueKey:   domain.String(bundle.RelationKeyTag.URL()),
			},
			{
				bundle.RelationKeyId:          domain.String(relationKey),
				bundle.RelationKeyName:        domain.String(relationKey),
				bundle.RelationKeyRelationKey: domain.String(relationKey),
				bundle.RelationKeySpaceId:     domain.String(spaceId),
				bundle.RelationKeyLayout:      domain.Int64(int64(model.ObjectType_relation)),
				bundle.RelationKeyUniqueKey:   domain.String(relationKeyUniqueKey.Marshal()),
			},
		})

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:     domain.String("id"),
			bundle.RelationKeyType:   domain.String(objectTypeId),
			bundle.RelationKeyLayout: domain.Int64(int64(model.ObjectType_set)),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyLayout.String(),
			Format: model.RelationFormat_number,
		})
		doc.Set(simple.New(&model.Block{
			Id:          "id",
			ChildrenIds: []string{"blockId"},
			Content:     &model.BlockContentOfSmartblock{Smartblock: &model.BlockContentSmartblock{}},
		}))

		doc.Set(simple.New(&model.Block{
			Id: "blockId",
			Content: &model.BlockContentOfDataview{Dataview: &model.BlockContentDataview{
				Views: []*model.BlockContentDataviewView{
					{
						Relations: []*model.BlockContentDataviewRelation{
							{
								Key: bundle.RelationKeyTag.String(),
							},
							{
								Key: relationKey,
							},
						},
					},
				},
			}},
		}))
		smartBlockTest.Doc = doc

		objectType := smarttest.New(objectTypeId)
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(objectTypeId),
			bundle.RelationKeyType: domain.String(objectTypeId),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		objectGetter := mock_cache.NewMockObjectGetter(t)
		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), objectTypeId).Return(objectType, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
			Format:        model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 3, len(expCtx.docs))
	})
	t.Run("objects without file", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		objectTypeId := "objectTypeId"
		objectTypeUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeObjectType, objectTypeId)
		assert.Nil(t, err)

		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:      domain.String("id"),
				bundle.RelationKeyName:    domain.String("name1"),
				bundle.RelationKeySpaceId: domain.String(spaceId),
				bundle.RelationKeyType:    domain.String(objectTypeId),
				bundle.RelationKeyLayout:  domain.Int64(int64(model.ObjectType_set)),
			},
			{
				bundle.RelationKeyId:        domain.String(objectTypeId),
				bundle.RelationKeyUniqueKey: domain.String(objectTypeUniqueKey.Marshal()),
				bundle.RelationKeyLayout:    domain.Int64(int64(model.ObjectType_objectType)),
				bundle.RelationKeySpaceId:   domain.String(spaceId),
				bundle.RelationKeyType:      domain.String(objectTypeId),
			},
		})

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:     domain.String("id"),
			bundle.RelationKeyType:   domain.String(objectTypeId),
			bundle.RelationKeyLayout: domain.Int64(int64(model.ObjectType_set)),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyLayout.String(),
			Format: model.RelationFormat_number,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New(objectTypeId)
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(objectTypeId),
			bundle.RelationKeyType: domain.String(objectTypeId),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		objectGetter := mock_cache.NewMockObjectGetter(t)
		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), objectTypeId).Return(objectType, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
			IncludeFiles:  true,
			Format:        model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 2, len(expCtx.docs))
	})
	t.Run("objects without file, not protobuf export", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		objectTypeId := "objectTypeId"
		objectTypeUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeObjectType, objectTypeId)
		assert.Nil(t, err)

		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:      domain.String("id"),
				bundle.RelationKeyName:    domain.String("name1"),
				bundle.RelationKeySpaceId: domain.String(spaceId),
				bundle.RelationKeyType:    domain.String(objectTypeId),
				bundle.RelationKeyLayout:  domain.Int64(int64(model.ObjectType_set)),
			},
			{
				bundle.RelationKeyId:        domain.String(objectTypeId),
				bundle.RelationKeyUniqueKey: domain.String(objectTypeUniqueKey.Marshal()),
				bundle.RelationKeyLayout:    domain.Int64(int64(model.ObjectType_objectType)),
				bundle.RelationKeySpaceId:   domain.String(spaceId),
				bundle.RelationKeyType:      domain.String(objectTypeId),
			},
		})

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String("id"),
			bundle.RelationKeyType: domain.String(objectTypeId),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		})
		smartBlockTest.Doc = doc

		objectGetter := mock_cache.NewMockObjectGetter(t)
		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:       spaceId,
			ObjectIds:     []string{"id"},
			IncludeNested: true,
			IncludeFiles:  true,
			Format:        model.Export_Markdown,
		})

		// when
		err = expCtx.docsForExport()

		// then
		assert.Nil(t, err)
		assert.Equal(t, 1, len(expCtx.docs))
	})

	t.Run("get derived objects - relation, object type with recommended relations, template with link", func(t *testing.T) {
		// given
		storeFixture := objectstore.NewStoreFixture(t)
		objectTypeKey := "customObjectType"
		objectTypeUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeObjectType, objectTypeKey)
		assert.Nil(t, err)

		relationKey := "key"
		uniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeRelation, relationKey)
		assert.Nil(t, err)

		recommendedRelationKey := "recommendedRelationKey"
		recommendedRelationUniqueKey, err := domain.NewUniqueKey(smartblock.SmartBlockTypeRelation, recommendedRelationKey)
		assert.Nil(t, err)

		relationObjectTypeKey := "relation"
		relationObjectTypeUK, err := domain.NewUniqueKey(smartblock.SmartBlockTypeObjectType, relationObjectTypeKey)
		assert.Nil(t, err)

		storeFixture.AddObjects(t, spaceId, []objectstore.TestObject{
			{
				bundle.RelationKeyId:      domain.String("id"),
				bundle.RelationKeySetOf:   domain.StringList([]string{relationKey}),
				bundle.RelationKeyType:    domain.String(objectTypeKey),
				bundle.RelationKeySpaceId: domain.String(spaceId),
			},
			{
				bundle.RelationKeyId:          domain.String(relationKey),
				bundle.RelationKeyRelationKey: domain.String(relationKey),
				bundle.RelationKeyUniqueKey:   domain.String(uniqueKey.Marshal()),
				bundle.RelationKeyLayout:      domain.Int64(int64(model.ObjectType_relation)),
				bundle.RelationKeySpaceId:     domain.String(spaceId),
				bundle.RelationKeyType:        domain.String(relationObjectTypeKey),
			},
			{
				bundle.RelationKeyId:                   domain.String(objectTypeKey),
				bundle.RelationKeyUniqueKey:            domain.String(objectTypeUniqueKey.Marshal()),
				bundle.RelationKeyLayout:               domain.Int64(int64(model.ObjectType_objectType)),
				bundle.RelationKeyRecommendedRelations: domain.StringList([]string{recommendedRelationKey}),
				bundle.RelationKeySpaceId:              domain.String(spaceId),
				bundle.RelationKeyType:                 domain.String(objectTypeKey),
			},
			{
				bundle.RelationKeyId:        domain.String(relationObjectTypeKey),
				bundle.RelationKeyUniqueKey: domain.String(relationObjectTypeUK.Marshal()),
				bundle.RelationKeyLayout:    domain.Int64(int64(model.ObjectType_objectType)),
				bundle.RelationKeySpaceId:   domain.String(spaceId),
				bundle.RelationKeyType:      domain.String(objectTypeKey),
			},
			{
				bundle.RelationKeyId:          domain.String(recommendedRelationKey),
				bundle.RelationKeyRelationKey: domain.String(recommendedRelationKey),
				bundle.RelationKeyUniqueKey:   domain.String(recommendedRelationUniqueKey.Marshal()),
				bundle.RelationKeyLayout:      domain.Int64(int64(model.ObjectType_relation)),
				bundle.RelationKeySpaceId:     domain.String(spaceId),
			},
		})

		objectGetter := mock_cache.NewMockObjectGetter(t)

		smartBlockTest := smarttest.New("id")
		doc := smartBlockTest.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:    domain.String("id"),
			bundle.RelationKeySetOf: domain.StringList([]string{relationKey}),
			bundle.RelationKeyType:  domain.String(objectTypeKey),
		}))
		doc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		})
		smartBlockTest.Doc = doc

		objectType := smarttest.New(objectTypeKey)
		objectTypeDoc := objectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(objectTypeKey),
			bundle.RelationKeyType: domain.String(objectTypeKey),
		}))
		objectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		objectType.Doc = objectTypeDoc

		relationObject := smarttest.New(relationKey)
		relationObjectDoc := relationObject.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(relationKey),
			bundle.RelationKeyType: domain.String(relationObjectTypeKey),
		}))
		relationObjectDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		relationObject.Doc = relationObjectDoc

		relationObjectType := smarttest.New(relationObjectTypeKey)
		relationObjectTypeDoc := relationObjectType.NewState().SetDetails(domain.NewDetailsFromMap(map[domain.RelationKey]domain.Value{
			bundle.RelationKeyId:   domain.String(relationObjectTypeKey),
			bundle.RelationKeyType: domain.String(objectTypeKey),
		}))
		relationObjectTypeDoc.AddRelationLinks(&model.RelationLink{
			Key:    bundle.RelationKeyId.String(),
			Format: model.RelationFormat_longtext,
		}, &model.RelationLink{
			Key:    bundle.RelationKeyType.String(),
			Format: model.RelationFormat_longtext,
		})
		relationObjectType.Doc = relationObjectTypeDoc

		objectGetter.EXPECT().GetObject(context.Background(), "id").Return(smartBlockTest, nil)
		objectGetter.EXPECT().GetObject(context.Background(), objectTypeKey).Return(objectType, nil)
		objectGetter.EXPECT().GetObject(context.Background(), relationKey).Return(relationObject, nil)
		objectGetter.EXPECT().GetObject(context.Background(), relationObjectTypeKey).Return(relationObjectType, nil)

		e := &export{
			objectStore: storeFixture,
			picker:      objectGetter,
		}

		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{
			SpaceId:   spaceId,
			ObjectIds: []string{"id"},
			Format:    model.Export_Protobuf,
		})

		// when
		err = expCtx.docsForExport()
		// then
		assert.Nil(t, err)
		assert.Equal(t, 5, len(expCtx.docs))
	})
}

func Test_provideFileName(t *testing.T) {
	t.Run("file dir for relation", func(t *testing.T) {
		// when
		fileName := makeFileName("docId", spaceId, pbjson.NewConverter(nil).Ext(), nil, smartblock.SmartBlockTypeRelation)

		// then
		assert.Equal(t, relationsDirectory+string(filepath.Separator)+"docId.pb.json", fileName)
	})
	t.Run("file dir for relation option", func(t *testing.T) {
		// when
		fileName := makeFileName("docId", spaceId, pbjson.NewConverter(nil).Ext(), nil, smartblock.SmartBlockTypeRelationOption)

		// then
		assert.Equal(t, relationsOptionsDirectory+string(filepath.Separator)+"docId.pb.json", fileName)
	})
	t.Run("file dir for types", func(t *testing.T) {
		// when
		fileName := makeFileName("docId", spaceId, pbjson.NewConverter(nil).Ext(), nil, smartblock.SmartBlockTypeObjectType)

		// then
		assert.Equal(t, typesDirectory+string(filepath.Separator)+"docId.pb.json", fileName)
	})
	t.Run("file dir for objects", func(t *testing.T) {
		// when
		fileName := makeFileName("docId", spaceId, pbjson.NewConverter(nil).Ext(), nil, smartblock.SmartBlockTypePage)

		// then
		assert.Equal(t, objectsDirectory+string(filepath.Separator)+"docId.pb.json", fileName)
	})
	t.Run("file dir for files objects", func(t *testing.T) {
		// when
		fileName := makeFileName("docId", spaceId, pbjson.NewConverter(nil).Ext(), nil, smartblock.SmartBlockTypeFileObject)

		// then
		assert.Equal(t, FilesObjects+string(filepath.Separator)+"docId.pb.json", fileName)
	})
	t.Run("space is not provided", func(t *testing.T) {
		// given
		st := state.NewDoc("root", nil).(*state.State)
		st.SetDetail(bundle.RelationKeySpaceId, domain.String(spaceId))

		// when
		fileName := makeFileName("docId", "", pbjson.NewConverter(st).Ext(), st, smartblock.SmartBlockTypeFileObject)

		// then
		assert.Equal(t, spaceDirectory+string(filepath.Separator)+spaceId+string(filepath.Separator)+FilesObjects+string(filepath.Separator)+"docId.pb.json", fileName)
	})
}

func Test_queryObjectsFromStoreByIds(t *testing.T) {
	t.Run("query 10 objects", func(t *testing.T) {
		// given
		store := objectstore.NewStoreFixture(t)
		ids := make([]string, 0, 10)
		for i := 0; i < 10; i++ {
			id := fmt.Sprintf("%d", i)
			store.AddObjects(t, spaceId, []objectstore.TestObject{
				{
					bundle.RelationKeyId:      domain.String(id),
					bundle.RelationKeySpaceId: domain.String(spaceId),
				},
			})
			ids = append(ids, id)
		}
		e := &export{objectStore: store}
		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{})

		// when
		records, err := expCtx.queryAndFilterObjectsByRelation(spaceId, ids, bundle.RelationKeyId)

		// then
		assert.Nil(t, err)
		assert.Len(t, records, 10)
	})
	t.Run("query 2000 objects", func(t *testing.T) {
		// given
		fixture := objectstore.NewStoreFixture(t)
		ids := make([]string, 0, 2000)
		for i := 0; i < 2000; i++ {
			id := fmt.Sprintf("%d", i)
			fixture.AddObjects(t, spaceId, []objectstore.TestObject{
				{
					bundle.RelationKeyId:      domain.String(id),
					bundle.RelationKeySpaceId: domain.String(spaceId),
				},
			})
			ids = append(ids, id)
		}
		e := &export{objectStore: fixture}
		expCtx := newExportContext(e, pb.RpcObjectListExportRequest{})

		// when
		records, err := expCtx.queryAndFilterObjectsByRelation(spaceId, ids, bundle.RelationKeyId)

		// then
		assert.Nil(t, err)
		assert.Len(t, records, 2000)
	})
}
