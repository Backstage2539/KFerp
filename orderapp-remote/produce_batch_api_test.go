package main

import "testing"

// DEV-044 API test cases (TODO state):
// We keep these as explicit skeletons first, then wire full httptest + test DB fixtures.

func TestProduceBatchAPI_Create_Success_TODO(t *testing.T) {
	t.Skip("TODO: POST /api/produce/batch/create should return 200 with batch_id and aggregated summary")
}

func TestProduceBatchAPI_Create_DuplicateOrderItemConflict_TODO(t *testing.T) {
	t.Skip("TODO: second create with same order_item should return 409")
}

func TestProduceBatchAPI_Create_EmptyOrderIDs_BadRequest_TODO(t *testing.T) {
	t.Skip("TODO: empty order_ids should return 400")
}

func TestProduceBatchAPI_Create_InvalidOrIneligibleOrders_BadRequest_TODO(t *testing.T) {
	t.Skip("TODO: non-unproduced or invalid order ids should return 400")
}

func TestProduceBatchAPI_List_Success_TODO(t *testing.T) {
	t.Skip("TODO: GET /api/produce/batch/list should return 200 and list items")
}

func TestProduceBatchAPI_Detail_Success_TODO(t *testing.T) {
	t.Skip("TODO: GET /api/produce/batch/:batch_id should return 200 with orders + summary")
}

func TestProduceBatchAPI_Detail_NotFound_TODO(t *testing.T) {
	t.Skip("TODO: non-existing batch_id should return 404")
}
