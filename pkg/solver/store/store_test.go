//go:build integration && solver

package store_test

import (
	"fmt"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/lilypad-tech/lilypad/pkg/data"
	"github.com/lilypad-tech/lilypad/pkg/solver/store"
	solverstore "github.com/lilypad-tech/lilypad/pkg/solver/store"
	databasestore "github.com/lilypad-tech/lilypad/pkg/solver/store/db"
	memorystore "github.com/lilypad-tech/lilypad/pkg/solver/store/memory"
	"golang.org/x/exp/rand"
)

const DB_CONN_STR = "postgres://postgres:postgres@localhost:5432/solver-db?sslmode=disable"

// Job offers

func TestJobOfferOps(t *testing.T) {
	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		// Test multiple job offers in a single test run
		t.Run(config.name, func(t *testing.T) {
			getStore, clearStore := config.init()
			store := getStore()
			defer clearStore()

			// Generate multiple job offers
			jobOffers := generateJobOffers(5, 50)

			for _, jobOffer := range jobOffers {
				// Add job offer
				added, err := store.AddJobOffer(jobOffer)
				if err != nil {
					t.Fatalf("Failed to add job offer: %v", err)
				}
				if added.ID != jobOffer.ID {
					t.Errorf("Expected ID %s, got %s", jobOffer.ID, added.ID)
				}

				// Get job offer
				retrieved, err := store.GetJobOffer(jobOffer.ID)
				if err != nil {
					t.Fatalf("Failed to get job offer: %v", err)
				}
				if retrieved == nil {
					t.Fatalf("Expected job offer, got nil")
				}
				if retrieved.ID != jobOffer.ID {
					t.Errorf("Expected ID %s, got %s", jobOffer.ID, retrieved.ID)
				}

				// Update job offer
				newDealID := generateCID()
				newState := generateState()

				updated, err := store.UpdateJobOfferState(jobOffer.ID, newDealID, newState)
				if err != nil {
					t.Fatalf("Failed to update job offer state: %v", err)
				}
				if updated.DealID != newDealID || updated.State != newState {
					t.Errorf("Update failed: expected dealID=%s state=%d, got dealID=%s state=%d",
						newDealID, newState, updated.DealID, updated.State)
				}

				// Remove job offer
				err = store.RemoveJobOffer(jobOffer.ID)
				if err != nil {
					t.Fatalf("Failed to remove job offer: %v", err)
				}

				// Verify removal
				removed, err := store.GetJobOffer(jobOffer.ID)
				if err != nil {
					t.Fatalf("Error checking removed job offer: %v", err)
				}
				if removed != nil {
					t.Error("Job offer still exists after removal")
				}
			}
		})
	}
}

func TestJobOfferQuery(t *testing.T) {
	// Test cases set offer fields relevant to querying.
	// All other fields are left with their zero-values.
	testCases := []struct {
		name     string
		offers   []data.JobOfferContainer
		query    store.GetJobOffersQuery
		expected []string // expected IDs in results
	}{
		{
			name: "filter by job creator",
			offers: []data.JobOfferContainer{
				{
					ID:         "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "",
					State:      0,
				},
				{
					ID:         "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					JobCreator: "0xabcdef0123456789abcdef0123456789abcdef01",
					DealID:     "",
					State:      0,
				},
			},
			query: store.GetJobOffersQuery{
				JobCreator: "0x1234567890123456789012345678901234567890",
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
		{
			name: "filter not matched offers",
			offers: []data.JobOfferContainer{
				{
					ID:         "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "QmZ9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kz",
					State:      0,
				},
				{
					ID:         "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "",
					State:      0,
				},
			},
			query: store.GetJobOffersQuery{
				NotMatched: true,
			},
			expected: []string{"QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky"},
		},
		{
			name: "filter out cancelled offers",
			offers: []data.JobOfferContainer{
				{
					ID:         "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "",
					State:      data.GetDefaultAgreementState(),
				},
				{
					ID:         "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "",
					State:      data.GetAgreementStateIndex("JobOfferCancelled"),
				},
			},
			query: store.GetJobOffersQuery{
				IncludeCancelled: false,
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
		{
			name: "include cancelled offers",
			offers: []data.JobOfferContainer{
				{
					ID:         "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "",
					State:      data.GetDefaultAgreementState(),
				},
				{
					ID:         "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "",
					State:      data.GetAgreementStateIndex("JobOfferCancelled"),
				},
			},
			query: store.GetJobOffersQuery{
				IncludeCancelled: true,
			},
			expected: []string{
				"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
				"QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
			},
		},
		{
			name: "combined filters",
			offers: []data.JobOfferContainer{
				{
					ID:         "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "",
					State:      data.GetDefaultAgreementState(),
				},
				{
					ID:         "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					JobCreator: "0xabcdef0123456789abcdef0123456789abcdef01",
					DealID:     "",
					State:      data.GetDefaultAgreementState(),
				},
				{
					ID:         "QmZ9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kz",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "QmW9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kw",
					State:      data.GetDefaultAgreementState(),
				},
				{
					ID:         "QmV9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kv",
					JobCreator: "0x1234567890123456789012345678901234567890",
					DealID:     "",
					State:      data.GetAgreementStateIndex("JobOfferCancelled"),
				},
			},
			query: store.GetJobOffersQuery{
				JobCreator:       "0x1234567890123456789012345678901234567890",
				NotMatched:       true,
				IncludeCancelled: false,
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
	}

	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		getStore, clearStore := config.init()
		defer clearStore()

		for _, tc := range testCases {
			// Test each case in a separate test run
			t.Run(fmt.Sprintf("%s/%s", config.name, tc.name), func(t *testing.T) {
				store := getStore()

				// Add test job offers
				for _, jo := range tc.offers {
					_, err := store.AddJobOffer(jo)
					if err != nil {
						t.Fatalf("Failed to add job offer: %v", err)
					}
				}

				// Run query
				results, err := store.GetJobOffers(tc.query)
				if err != nil {
					t.Fatalf("GetJobOffers failed: %v", err)
				}

				// Extract result IDs
				resultIDs := make([]string, len(results))
				for i, r := range results {
					resultIDs[i] = r.ID
				}

				// Sort both slices for comparison
				sort.Strings(resultIDs)
				sort.Strings(tc.expected)

				if !slices.Equal(resultIDs, tc.expected) {
					t.Errorf("Expected results %v, got %v", tc.expected, resultIDs)
				}
			})
		}
	}
}

// Resource Offer

func TestResourceOfferOps(t *testing.T) {
	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		// Test multiple resource offers in a single test run
		t.Run(config.name, func(t *testing.T) {
			getStore, clearStore := config.init()
			store := getStore()
			defer clearStore()

			// Generate multiple resource offers
			resourceOffers := generateResourceOffers(5, 50)

			for _, resourceOffer := range resourceOffers {
				// Add resource offer
				added, err := store.AddResourceOffer(resourceOffer)
				if err != nil {
					t.Fatalf("Failed to add resource offer: %v", err)
				}
				if added.ID != resourceOffer.ID {
					t.Errorf("Expected ID %s, got %s", resourceOffer.ID, added.ID)
				}

				// Get resource offer
				retrieved, err := store.GetResourceOffer(resourceOffer.ID)
				if err != nil {
					t.Fatalf("Failed to get resource offer: %v", err)
				}
				if retrieved == nil {
					t.Fatalf("Expected resource offer, got nil")
				}
				if retrieved.ID != resourceOffer.ID {
					t.Errorf("Expected ID %s, got %s", resourceOffer.ID, retrieved.ID)
				}

				// Get resource offer by address
				byAddress, err := store.GetResourceOfferByAddress(resourceOffer.ResourceProvider)
				if err != nil {
					t.Fatalf("Failed to get resource offer by address: %v", err)
				}
				if byAddress == nil {
					t.Fatalf("Expected resource offer by address, got nil")
				}
				if byAddress.ResourceProvider != resourceOffer.ResourceProvider {
					t.Errorf("Expected provider %s, got %s", resourceOffer.ResourceProvider, byAddress.ResourceProvider)
				}

				// Update resource offer
				newDealID := generateCID()
				newState := generateState()

				updated, err := store.UpdateResourceOfferState(resourceOffer.ID, newDealID, newState)
				if err != nil {
					t.Fatalf("Failed to update resource offer state: %v", err)
				}
				if updated.DealID != newDealID || updated.State != newState {
					t.Errorf("Update failed: expected dealID=%s state=%d, got dealID=%s state=%d",
						newDealID, newState, updated.DealID, updated.State)
				}

				// Remove resource offer
				err = store.RemoveResourceOffer(resourceOffer.ID)
				if err != nil {
					t.Fatalf("Failed to remove resource offer: %v", err)
				}

				// Verify removal
				removed, err := store.GetResourceOffer(resourceOffer.ID)
				if err != nil {
					t.Fatalf("Error checking removed resource offer: %v", err)
				}
				if removed != nil {
					t.Error("Resource offer still exists after removal")
				}

				// Verify removal by address
				removedByAddr, err := store.GetResourceOfferByAddress(resourceOffer.ResourceProvider)
				if err != nil {
					t.Fatalf("Error checking removed resource offer by address: %v", err)
				}
				if removedByAddr != nil {
					t.Error("Resource offer still exists after removal when checking by address")
				}
			}
		})
	}
}

func TestResourceOfferQuery(t *testing.T) {
	// Test cases set offer fields relevant to querying.
	// All other fields are left with their zero-values.
	testCases := []struct {
		name     string
		offers   []data.ResourceOfferContainer
		query    store.GetResourceOffersQuery
		expected []string // expected IDs in results
	}{
		{
			name: "filter by resource provider",
			offers: []data.ResourceOfferContainer{
				{
					ID:               "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "",
					State:            data.GetDefaultAgreementState(),
				},
				{
					ID:               "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					ResourceProvider: "0xabcdef0123456789abcdef0123456789abcdef01",
					DealID:           "",
					State:            data.GetDefaultAgreementState(),
				},
			},
			query: store.GetResourceOffersQuery{
				ResourceProvider: "0x1234567890123456789012345678901234567890",
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
		{
			name: "filter not matched offers",
			offers: []data.ResourceOfferContainer{
				{
					ID:               "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "QmZ9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kz",
					State:            data.GetDefaultAgreementState(),
				},
				{
					ID:               "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "",
					State:            data.GetDefaultAgreementState(),
				},
			},
			query: store.GetResourceOffersQuery{
				NotMatched: true,
			},
			expected: []string{"QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky"},
		},
		{
			name: "filter active offers",
			offers: []data.ResourceOfferContainer{
				{
					ID:               "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "",
					State:            data.GetAgreementStateIndex("DealNegotiating"),
				},
				{
					ID:               "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "",
					State:            data.GetAgreementStateIndex("DealAgreed"),
				},
				{
					ID:               "QmZ9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kz",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "",
					State:            data.GetAgreementStateIndex("ResultsSubmitted"),
				},
				{
					ID:               "QmV9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kv",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "",
					State:            data.GetAgreementStateIndex("ResultsAccepted"),
				},
			},
			query: store.GetResourceOffersQuery{
				Active: true,
			},
			expected: []string{
				"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
				"QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
			},
		},
		{
			name: "combined filters",
			offers: []data.ResourceOfferContainer{
				{
					ID:               "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "",
					State:            data.GetAgreementStateIndex("DealNegotiating"),
				},
				{
					ID:               "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					ResourceProvider: "0xabcdef0123456789abcdef0123456789abcdef01",
					DealID:           "",
					State:            data.GetAgreementStateIndex("DealNegotiating"),
				},
				{
					ID:               "QmZ9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kz",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "QmW9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kw",
					State:            data.GetAgreementStateIndex("DealNegotiating"),
				},
				{
					ID:               "QmV9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kv",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					DealID:           "",
					State:            data.GetAgreementStateIndex("ResultsSubmitted"),
				},
			},
			query: store.GetResourceOffersQuery{
				ResourceProvider: "0x1234567890123456789012345678901234567890",
				NotMatched:       true,
				Active:           true,
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
	}

	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		getStore, clearStore := config.init()
		defer clearStore()

		for _, tc := range testCases {
			// Test each case in a separate test run
			t.Run(fmt.Sprintf("%s/%s", config.name, tc.name), func(t *testing.T) {
				store := getStore()

				// Add test resource offers
				for _, ro := range tc.offers {
					_, err := store.AddResourceOffer(ro)
					if err != nil {
						t.Fatalf("Failed to add resource offer: %v", err)
					}
				}

				// Run query
				results, err := store.GetResourceOffers(tc.query)
				if err != nil {
					t.Fatalf("GetResourceOffers failed: %v", err)
				}

				// Extract result IDs
				resultIDs := make([]string, len(results))
				for i, r := range results {
					resultIDs[i] = r.ID
				}

				// Sort both slices for comparison
				sort.Strings(resultIDs)
				sort.Strings(tc.expected)

				if !slices.Equal(resultIDs, tc.expected) {
					t.Errorf("Expected results %v, got %v", tc.expected, resultIDs)
				}
			})
		}
	}
}

// Deals

func TestDealOps(t *testing.T) {
	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		// Test multiple deals in a single test run
		t.Run(config.name, func(t *testing.T) {
			getStore, clearStore := config.init()
			store := getStore()
			defer clearStore()

			// Generate multiple deals
			deals := generateDeals(5, 50)

			for _, deal := range deals {
				// Add deal
				added, err := store.AddDeal(deal)
				if err != nil {
					t.Fatalf("Failed to add deal: %v", err)
				}
				if added.ID != deal.ID {
					t.Errorf("Expected ID %s, got %s", deal.ID, added.ID)
				}

				// Get deal
				retrieved, err := store.GetDeal(deal.ID)
				if err != nil {
					t.Fatalf("Failed to get deal: %v", err)
				}
				if retrieved == nil {
					t.Fatalf("Expected deal, got nil")
				}
				if retrieved.ID != deal.ID {
					t.Errorf("Expected ID %s, got %s", deal.ID, retrieved.ID)
				}

				// Remove deal
				err = store.RemoveDeal(deal.ID)
				if err != nil {
					t.Fatalf("Failed to remove deal: %v", err)
				}

				// Verify removal
				removed, err := store.GetDeal(deal.ID)
				if err != nil {
					t.Fatalf("Error checking removed deal: %v", err)
				}
				if removed != nil {
					t.Error("Deal still exists after removal")
				}
			}
		})
	}
}

func TestDealGetAll(t *testing.T) {
	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		// Test batch of deals in a test run
		t.Run(config.name, func(t *testing.T) {
			getStore, clearStore := config.init()
			store := getStore()
			defer clearStore()

			// Generate multiple deals or no deals
			deals := generateDeals(0, 10)
			addedIDs := make([]string, len(deals))

			// Add deals
			for i, deal := range deals {
				added, err := store.AddDeal(deal)
				if err != nil {
					t.Fatalf("Failed to add deal: %v", err)
				}
				addedIDs[i] = added.ID
			}

			// Get all deals
			allDeals, err := store.GetDealsAll()
			if err != nil {
				t.Fatalf("Failed to get all deals: %v", err)
			}

			// Verify count matches
			if len(allDeals) != len(deals) {
				t.Errorf("Expected %d deals, got %d", len(deals), len(allDeals))
			}

			// Verify all added deals are present
			retrievedIDs := make([]string, len(allDeals))
			for i, deal := range allDeals {
				retrievedIDs[i] = deal.ID
			}

			// Sort both slices for comparison
			sort.Strings(addedIDs)
			sort.Strings(retrievedIDs)

			if !slices.Equal(retrievedIDs, addedIDs) {
				t.Errorf("Retrieved deals don't match added deals.\nAdded: %v\nRetrieved: %v",
					addedIDs, retrievedIDs)
			}
		})
	}
}

func TestDealUpdates(t *testing.T) {
	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		// Test multiple deals in a single test run
		t.Run(config.name, func(t *testing.T) {
			getStore, clearStore := config.init()
			store := getStore()
			defer clearStore()

			// Generate multiple deals
			deals := generateDeals(5, 50)

			for _, deal := range deals {
				// Add deal
				added, err := store.AddDeal(deal)
				if err != nil {
					t.Fatalf("Failed to add deal: %v", err)
				}

				// Update deal state
				newState := generateState()
				updated, err := store.UpdateDealState(added.ID, newState)
				if err != nil {
					t.Fatalf("Failed to update deal state: %v", err)
				}
				if updated.State != newState {
					t.Errorf("Update state failed: expected state=%d, got state=%d",
						newState, updated.State)
				}

				// Update deal mediator
				newMediator := generateEthAddress()
				updated, err = store.UpdateDealMediator(added.ID, newMediator)
				if err != nil {
					t.Fatalf("Failed to update deal mediator: %v", err)
				}
				if updated.Mediator != newMediator {
					t.Errorf("Update mediator failed: expected mediator=%s, got mediator=%s",
						newMediator, updated.Mediator)
				}

				// Update deal job creator transactions
				jcTxs := data.DealTransactionsJobCreator{
					Agree:                generateEthTxHash(),
					AcceptResult:         generateEthTxHash(),
					CheckResult:          generateEthTxHash(),
					TimeoutAgree:         generateEthTxHash(),
					TimeoutSubmitResult:  generateEthTxHash(),
					TimeoutMediateResult: generateEthTxHash(),
				}
				updated, err = store.UpdateDealTransactionsJobCreator(added.ID, jcTxs)
				if err != nil {
					t.Fatalf("Failed to update job creator transactions: %v", err)
				}
				if updated.Transactions.JobCreator != jcTxs {
					t.Error("Job creator transactions not updated correctly")
				}

				// Update deal transactions resource provider
				rpTxs := data.DealTransactionsResourceProvider{
					Agree:                generateEthTxHash(),
					AddResult:            generateEthTxHash(),
					TimeoutAgree:         generateEthTxHash(),
					TimeoutJudgeResult:   generateEthTxHash(),
					TimeoutMediateResult: generateEthTxHash(),
				}
				updated, err = store.UpdateDealTransactionsResourceProvider(added.ID, rpTxs)
				if err != nil {
					t.Fatalf("Failed to update resource provider transactions: %v", err)
				}
				if updated.Transactions.ResourceProvider != rpTxs {
					t.Error("Resource provider transactions not updated correctly")
				}

				// Update deal transactions mediator
				mediatorTxs := data.DealTransactionsMediator{
					MediationAcceptResult: generateEthTxHash(),
					MediationRejectResult: generateEthTxHash(),
				}
				updatedMediatorTxs, err := store.UpdateDealTransactionsMediator(added.ID, mediatorTxs)
				if err != nil {
					t.Fatalf("Failed to update mediator transactions: %v", err)
				}
				if updatedMediatorTxs.Transactions.Mediator != mediatorTxs {
					t.Error("Mediator transactions not updated correctly")
				}
			}
		})
	}
}

func TestDealQuery(t *testing.T) {
	// Test cases set deal fields relevant to querying.
	// All other fields are left with their zero-values.
	testCases := []struct {
		name     string
		deals    []data.DealContainer
		query    store.GetDealsQuery
		expected []string // expected IDs in results
	}{
		{
			name: "filter by job creator",
			deals: []data.DealContainer{
				{
					ID:         "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					JobCreator: "0x1234567890123456789012345678901234567890",
					State:      data.GetDefaultAgreementState(),
				},
				{
					ID:         "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					JobCreator: "0xabcdef0123456789abcdef0123456789abcdef01",
					State:      data.GetDefaultAgreementState(),
				},
			},
			query: store.GetDealsQuery{
				JobCreator: "0x1234567890123456789012345678901234567890",
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
		{
			name: "filter by resource provider",
			deals: []data.DealContainer{
				{
					ID:               "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					ResourceProvider: "0x1234567890123456789012345678901234567890",
					State:            data.GetDefaultAgreementState(),
				},
				{
					ID:               "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					ResourceProvider: "0xabcdef0123456789abcdef0123456789abcdef01",
					State:            data.GetDefaultAgreementState(),
				},
			},
			query: store.GetDealsQuery{
				ResourceProvider: "0x1234567890123456789012345678901234567890",
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
		{
			name: "filter by mediator",
			deals: []data.DealContainer{
				{
					ID:       "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					Mediator: "0x1234567890123456789012345678901234567890",
					State:    data.GetDefaultAgreementState(),
				},
				{
					ID:       "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					Mediator: "0xabcdef0123456789abcdef0123456789abcdef01",
					State:    data.GetDefaultAgreementState(),
				},
			},
			query: store.GetDealsQuery{
				Mediator: "0x1234567890123456789012345678901234567890",
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
		{
			name: "filter by state",
			deals: []data.DealContainer{
				{
					ID:    "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					State: data.GetAgreementStateIndex("DealNegotiating"),
				},
				{
					ID:    "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					State: data.GetAgreementStateIndex("DealAgreed"),
				},
			},
			query: store.GetDealsQuery{
				State: "DealNegotiating",
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
		{
			name: "combined filters",
			deals: []data.DealContainer{
				{
					ID:               "QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx",
					JobCreator:       "0x1234567890123456789012345678901234567890",
					ResourceProvider: "0x2234567890123456789012345678901234567890",
					State:            data.GetAgreementStateIndex("DealNegotiating"),
				},
				{
					ID:               "QmX9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Ky",
					JobCreator:       "0x1234567890123456789012345678901234567890",
					ResourceProvider: "0x2234567890123456789012345678901234567890",
					State:            data.GetAgreementStateIndex("DealAgreed"),
				},
				{
					ID:               "QmZ9JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kz",
					JobCreator:       "0x1234567890123456789012345678901234567890",
					ResourceProvider: "0x3234567890123456789012345678901234567890",
					State:            data.GetAgreementStateIndex("DealNegotiating"),
				},
			},
			query: store.GetDealsQuery{
				JobCreator:       "0x1234567890123456789012345678901234567890",
				ResourceProvider: "0x2234567890123456789012345678901234567890",
				State:            "DealNegotiating",
			},
			expected: []string{"QmY8JwJh3bYDUuAnwfpxwStjUY1nQwyhJJ4SPpdV3bZ9Kx"},
		},
	}

	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		getStore, clearStore := config.init()
		defer clearStore()

		for _, tc := range testCases {
			// Test each case in a separate test run
			t.Run(fmt.Sprintf("%s/%s", config.name, tc.name), func(t *testing.T) {
				store := getStore()

				// Add deals
				for _, deal := range tc.deals {
					_, err := store.AddDeal(deal)
					if err != nil {
						t.Fatalf("Failed to add deal: %v", err)
					}
				}

				// Run query
				results, err := store.GetDeals(tc.query)
				if err != nil {
					t.Fatalf("GetDeals failed: %v", err)
				}

				// Extract result IDs
				resultIDs := make([]string, len(results))
				for i, r := range results {
					resultIDs[i] = r.ID
				}

				// Sort both slices for comparison
				sort.Strings(resultIDs)
				sort.Strings(tc.expected)

				if !slices.Equal(resultIDs, tc.expected) {
					t.Errorf("Expected results %v, got %v", tc.expected, resultIDs)
				}
			})
		}
	}
}

// Results

func TestResultOps(t *testing.T) {
	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		// Test multiple results in a single test run
		t.Run(config.name, func(t *testing.T) {
			getStore, clearStore := config.init()
			store := getStore()
			defer clearStore()

			// Generate multiple results
			results := generateResults(5, 50)
			addedResults := make(map[string]data.Result)

			// Add results
			for _, result := range results {
				added, err := store.AddResult(result)
				if err != nil {
					t.Fatalf("Failed to add result: %v", err)
				}
				if added.DealID != result.DealID {
					t.Errorf("Expected DealID %s, got %s", result.DealID, added.DealID)
				}
				if added.ID != result.ID {
					t.Errorf("Expected ID %s, got %s", result.ID, added.ID)
				}
				addedResults[result.DealID] = result
			}

			// Get results
			allResults, err := store.GetResults()
			if err != nil {
				t.Fatalf("Failed to get all results: %v", err)
			}

			// Verify count matches
			if len(allResults) != len(results) {
				t.Errorf("Expected %d results, got %d", len(results), len(allResults))
			}

			// Verify results are present and correct
			for _, result := range allResults {
				original, exists := addedResults[result.DealID]
				if !exists {
					t.Errorf("Got unexpected result with DealID %s", result.DealID)
					continue
				}
				if result.ID != original.ID {
					t.Errorf("Result ID mismatch for DealID %s: expected %s, got %s",
						result.DealID, original.ID, result.ID)
				}
			}

			// Test individual result operations
			for _, result := range results {
				// Get result by deal ID
				retrieved, err := store.GetResult(result.DealID)
				if err != nil {
					t.Fatalf("Failed to get result: %v", err)
				}
				if retrieved == nil {
					t.Fatalf("Expected result, got nil")
				}
				if retrieved.DealID != result.DealID {
					t.Errorf("Expected DealID %s, got %s", result.DealID, retrieved.DealID)
				}
				if retrieved.ID != result.ID {
					t.Errorf("Expected ID %s, got %s", result.ID, retrieved.ID)
				}

				// Remove result
				err = store.RemoveResult(result.DealID)
				if err != nil {
					t.Fatalf("Failed to remove result: %v", err)
				}

				// Verify removal
				removed, err := store.GetResult(result.DealID)
				if err != nil {
					t.Fatalf("Error checking removed result: %v", err)
				}
				if removed != nil {
					t.Error("Result still exists after removal")
				}
			}

			// Verify results were removed using GetResults
			finalResults, err := store.GetResults()
			if err != nil {
				t.Fatalf("Failed to get final results: %v", err)
			}
			if len(finalResults) != 0 {
				t.Errorf("Expected 0 results after removal, got %d", len(finalResults))
			}
		})
	}
}

// Match decisions

func TestMatchDecisionOps(t *testing.T) {
	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		// Test multiple match decisions in a single test run
		t.Run(config.name, func(t *testing.T) {
			getStore, clearStore := config.init()
			store := getStore()
			defer clearStore()

			// Generate match decisions
			decisions := generateMatchDecisions(5, 50)
			addedDecisions := make(map[string]*data.MatchDecision)

			// Add match decisions
			for _, decision := range decisions {
				// Add match decision
				added, err := store.AddMatchDecision(
					decision.ResourceOffer,
					decision.JobOffer,
					decision.Deal,
					decision.Result,
				)
				if err != nil {
					t.Fatalf("Failed to add match decision: %v", err)
				}
				if added.ResourceOffer != decision.ResourceOffer {
					t.Errorf("Expected ResourceOffer %s, got %s",
						decision.ResourceOffer, added.ResourceOffer)
				}
				if added.JobOffer != decision.JobOffer {
					t.Errorf("Expected JobOffer %s, got %s",
						decision.JobOffer, added.JobOffer)
				}
				if added.Deal != decision.Deal {
					t.Errorf("Expected Deal %s, got %s",
						decision.Deal, added.Deal)
				}
				if added.Result != decision.Result {
					t.Errorf("Expected Result %v, got %v",
						decision.Result, added.Result)
				}

				// Store for later verification
				matchID := solverstore.GetMatchID(decision.ResourceOffer, decision.JobOffer)
				addedDecisions[matchID] = added
			}

			// Get match decisions
			allDecisions, err := store.GetMatchDecisions()
			if err != nil {
				t.Fatalf("Failed to get all match decisions: %v", err)
			}

			// Verify count matches
			if len(allDecisions) != len(addedDecisions) {
				t.Errorf("Expected %d match decisions, got %d",
					len(addedDecisions), len(allDecisions))
			}

			// Verify decisions are present and correct
			for _, decision := range allDecisions {
				matchID := solverstore.GetMatchID(decision.ResourceOffer, decision.JobOffer)
				original, exists := addedDecisions[matchID]
				if !exists {
					t.Errorf("Got unexpected match decision for resource offer %s and job offer %s",
						decision.ResourceOffer, decision.JobOffer)
					continue
				}
				// Check deal and result. JobOffer and ResourceOffer in individual checks below.
				if decision.Deal != original.Deal {
					t.Errorf("Match decision Deal mismatch for ID %s: expected %s, got %s",
						matchID, original.Deal, decision.Deal)
				}
				if decision.Result != original.Result {
					t.Errorf("Match decision Result mismatch for ID %s: expected %v, got %v",
						matchID, original.Result, decision.Result)
				}
			}

			// Test individual match decision operations
			for _, decision := range decisions {
				// Get match decision
				retrieved, err := store.GetMatchDecision(
					decision.ResourceOffer,
					decision.JobOffer,
				)
				if err != nil {
					t.Fatalf("Failed to get match decision: %v", err)
				}
				if retrieved == nil {
					t.Fatal("Expected match decision, got nil")
				}
				if retrieved.ResourceOffer != decision.ResourceOffer {
					t.Errorf("Expected ResourceOffer %s, got %s",
						decision.ResourceOffer, retrieved.ResourceOffer)
				}
				if retrieved.JobOffer != decision.JobOffer {
					t.Errorf("Expected JobOffer %s, got %s",
						decision.JobOffer, retrieved.JobOffer)
				}

				// Remove match decision by job offer
				err = store.RemoveMatchDecision(decision.ResourceOffer, decision.JobOffer)
				if err != nil {
					t.Fatalf("Failed to remove match decision: %v", err)
				}

				// Verify removal
				removed, err := store.GetMatchDecision(
					decision.ResourceOffer,
					decision.JobOffer,
				)
				if err != nil {
					t.Fatalf("Error checking removed match decision: %v", err)
				}
				if removed != nil {
					t.Error("Match decision still exists after removal")
				}
			}
		})
	}
}

func TestMatchDecisionRemove(t *testing.T) {
	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		t.Run(config.name, func(t *testing.T) {
			getStore, clearStore := config.init()
			store := getStore()
			defer clearStore()

			matchDecision := data.MatchDecision{
				ResourceOffer: generateCID(),
				JobOffer:      generateCID(),
				Deal:          generateCID(),
				Result:        rand.Intn(2) == 1, // Generate random bool
			}

			testCases := []struct {
				name          string
				matchDecision data.MatchDecision
				removeBy      []string
			}{
				{
					name:          "remove by job offer",
					matchDecision: matchDecision,
					removeBy:      []string{"", matchDecision.JobOffer},
				},
				{
					name:          "remove by resource offer",
					matchDecision: matchDecision,
					removeBy:      []string{matchDecision.ResourceOffer, ""},
				},
				{
					name:          "remove by job offer and resource offer",
					matchDecision: matchDecision,
					removeBy:      []string{matchDecision.ResourceOffer, matchDecision.JobOffer},
				},
			}

			// Test removal by job creator, resource provider, or both
			for _, tc := range testCases {
				// Remove match decision
				err := store.RemoveMatchDecision(tc.removeBy[0], tc.removeBy[1])
				if err != nil {
					t.Fatalf("Failed to remove match decision: %v", err)
				}

				// Check removal
				retrieved, err := store.GetMatchDecision(tc.matchDecision.ResourceOffer, tc.matchDecision.JobOffer)
				if err != nil {
					t.Fatalf("Failed to get match decision: %v", err)
				}
				if retrieved != nil {
					t.Errorf("Match decision %s shouldn't exist but does",
						solverstore.GetMatchID(tc.matchDecision.ResourceOffer, tc.matchDecision.JobOffer))
				}
			}
		})
	}
}

// Concurrency for all

func TestConcurrentOps(t *testing.T) {
	jobOffers := generateJobOffers(4, 10)
	resourceOffers := generateResourceOffers(4, 10)
	deals := generateDeals(4, 10)
	results := generateResults(4, 10)
	matchDecisions := generateMatchDecisions(4, 10)

	storeConfigs := setupStores(t)
	for _, config := range storeConfigs {
		// Test concurrent adds in a single test run
		t.Run(config.name, func(t *testing.T) {
			getStore, clearStore := config.init()
			store := getStore()
			defer clearStore()

			count := len(jobOffers) + len(resourceOffers) + len(deals) + len(results) + len(matchDecisions)
			errCh := make(chan error, count)
			var wg sync.WaitGroup

			// Add job offers concurrently
			for _, offer := range jobOffers {
				wg.Add(1)
				go func(o data.JobOfferContainer) {
					defer wg.Done()
					_, err := store.AddJobOffer(o)
					if err != nil {
						errCh <- fmt.Errorf("job offer error: %v", err)
					}
				}(offer)
			}

			// Add resource offers concurrently
			for _, offer := range resourceOffers {
				wg.Add(1)
				go func(o data.ResourceOfferContainer) {
					defer wg.Done()
					_, err := store.AddResourceOffer(o)
					if err != nil {
						errCh <- fmt.Errorf("resource offer error: %v", err)
					}
				}(offer)
			}

			// Add deals concurrently
			for _, deal := range deals {
				wg.Add(1)
				go func(d data.DealContainer) {
					defer wg.Done()
					_, err := store.AddDeal(d)
					if err != nil {
						errCh <- fmt.Errorf("deal error: %v", err)
					}
				}(deal)
			}

			// Add results concurrently
			for _, result := range results {
				wg.Add(1)
				go func(r data.Result) {
					defer wg.Done()
					_, err := store.AddResult(r)
					if err != nil {
						errCh <- fmt.Errorf("result error: %v", err)
					}
				}(result)
			}

			// Add match decisions concurrently
			for _, decision := range matchDecisions {
				wg.Add(1)
				go func(d data.MatchDecision) {
					defer wg.Done()
					_, err := store.AddMatchDecision(d.ResourceOffer, d.JobOffer, d.Deal, d.Result)
					if err != nil {
						errCh <- fmt.Errorf("match decision error: %v", err)
					}
				}(decision)
			}

			wg.Wait()
			close(errCh)

			// Check for any errors during concurrent operations
			for err := range errCh {
				if err != nil {
					t.Errorf("Concurrent operation error: %v", err)
				}
			}

			// Verify all job offers were added
			for _, offer := range jobOffers {
				retrieved, err := store.GetJobOffer(offer.ID)
				if err != nil {
					t.Errorf("Failed to get job offer %s: %v", offer.ID, err)
				}
				if retrieved == nil {
					t.Errorf("Job offer %s not found after concurrent add", offer.ID)
				}
				if retrieved != nil && retrieved.ID != offer.ID {
					t.Errorf("Retrieved job offer ID mismatch: expected %s, got %s", offer.ID, retrieved.ID)
				}
			}

			// Verify all resource offers were added
			for _, offer := range resourceOffers {
				retrieved, err := store.GetResourceOffer(offer.ID)
				if err != nil {
					t.Errorf("Failed to get resource offer %s: %v", offer.ID, err)
				}
				if retrieved == nil {
					t.Errorf("Resource offer %s not found after concurrent add", offer.ID)
				}
				if retrieved != nil && retrieved.ID != offer.ID {
					t.Errorf("Retrieved resource offer ID mismatch: expected %s, got %s", offer.ID, retrieved.ID)
				}
			}

			// Verify all deals were added
			for _, deal := range deals {
				retrieved, err := store.GetDeal(deal.ID)
				if err != nil {
					t.Errorf("Failed to get deal %s: %v", deal.ID, err)
				}
				if retrieved == nil {
					t.Errorf("Deal %s not found after concurrent add", deal.ID)
				}
				if retrieved != nil && retrieved.ID != deal.ID {
					t.Errorf("Retrieved deal ID mismatch: expected %s, got %s", deal.ID, retrieved.ID)
				}
			}

			// Verify all results were added
			for _, result := range results {
				retrieved, err := store.GetResult(result.DealID)
				if err != nil {
					t.Errorf("Failed to get result for deal %s: %v", result.DealID, err)
				}
				if retrieved == nil {
					t.Errorf("Result for deal ID %s not found after concurrent add", result.DealID)
				}
				if retrieved != nil {
					if retrieved.DealID != result.DealID {
						t.Errorf("Retrieved result DealID mismatch: expected %s, got %s", result.DealID, retrieved.DealID)
					}
					if retrieved.ID != result.ID {
						t.Errorf("Retrieved result ID mismatch: expected %s, got %s", result.ID, retrieved.ID)
					}
				}
			}

			// Verify all match decisions were added
			for _, decision := range matchDecisions {
				retrieved, err := store.GetMatchDecision(decision.ResourceOffer, decision.JobOffer)
				if err != nil {
					t.Errorf("Failed to get match decision for resource offer %s and job offer %s: %v",
						decision.ResourceOffer, decision.JobOffer, err)
				}
				if retrieved == nil {
					t.Errorf("Match decision for resource offer %s and job offer %s not found after concurrent add",
						decision.ResourceOffer, decision.JobOffer)
				}
				if retrieved != nil {
					if retrieved.ResourceOffer != decision.ResourceOffer {
						t.Errorf("Retrieved match decision ResourceOffer mismatch: expected %s, got %s",
							decision.ResourceOffer, retrieved.ResourceOffer)
					}
					if retrieved.JobOffer != decision.JobOffer {
						t.Errorf("Retrieved match decision JobOffer mismatch: expected %s, got %s",
							decision.JobOffer, retrieved.JobOffer)
					}
					if retrieved.Deal != decision.Deal {
						t.Errorf("Retrieved match decision Deal mismatch: expected %s, got %s",
							decision.Deal, retrieved.Deal)
					}
					if retrieved.Result != decision.Result {
						t.Errorf("Retrieved match decision Result mismatch: expected %v, got %v",
							decision.Result, retrieved.Result)
					}
				}
			}

		})
	}
}

// Utilities

type storeConfig struct {
	name string
	init func() (getStore func() store.SolverStore, clearStore func())
}

func setupStores(t *testing.T) []storeConfig {
	initMemory := func() (func() store.SolverStore, func()) {
		// Get store function creates a new memory store
		// which effectively clears data between runs
		getStore := func() store.SolverStore {
			s, err := memorystore.NewSolverStoreMemory()
			if err != nil {
				t.Fatalf("Failed to create memory store: %v", err)
			}
			return s
		}
		clearStore := func() {}

		return getStore, clearStore
	}

	initDatabase := func() (func() store.SolverStore, func()) {
		db, err := databasestore.NewSolverStoreDatabase(DB_CONN_STR, "silent")
		if err != nil {
			t.Fatalf("Failed to create database store: %v", err)
		}

		// Clear data at initialization
		clearStoreDatabase(t, db)

		// Get store functions clear data and returns
		// the same store instance
		getStore := func() store.SolverStore {
			clearStoreDatabase(t, db)
			return db
		}
		clearStore := func() { clearStoreDatabase(t, db) }

		return getStore, clearStore
	}

	return []storeConfig{
		{name: "memory", init: initMemory},
		{name: "database", init: initDatabase},
	}
}

func clearStoreDatabase(t *testing.T, s store.SolverStore) {
	// Delete job offers
	jobOffers, err := s.GetJobOffers(store.GetJobOffersQuery{
		IncludeCancelled: true,
	})
	if err != nil {
		t.Fatalf("Failed to get existing job offers: %v", err)
	}

	for _, result := range jobOffers {
		err := s.RemoveJobOffer(result.ID)
		if err != nil {
			t.Fatalf("Failed to remove existing job offer: %v", err)
		}
	}

	// Delete resource offers
	resourceOffers, err := s.GetResourceOffers(store.GetResourceOffersQuery{})
	if err != nil {
		t.Fatalf("Failed to get existing resource offers: %v", err)
	}

	for _, result := range resourceOffers {
		err := s.RemoveResourceOffer(result.ID)
		if err != nil {
			t.Fatalf("Failed to remove existing resource offer: %v", err)
		}
	}

	// Delete deals
	deals, err := s.GetDealsAll()
	if err != nil {
		t.Fatalf("Failed to get existing deals: %v", err)
	}

	for _, deal := range deals {
		err := s.RemoveDeal(deal.ID)
		if err != nil {
			t.Fatalf("Failed to remove existing deal: %v", err)
		}
	}

	// Delete results
	results, err := s.GetResults()
	if err != nil {
		t.Fatalf("Failed to get existing results: %v", err)
	}

	for _, result := range results {
		err := s.RemoveResult(result.DealID)
		if err != nil {
			t.Fatalf("Failed to remove existing result: %v", err)
		}
	}

	// Delete match decisions
	decisions, err := s.GetMatchDecisions()
	if err != nil {
		t.Fatalf("Failed to get existing match decisions: %v", err)
	}

	for _, decision := range decisions {
		err := s.RemoveMatchDecision(decision.ResourceOffer, decision.JobOffer)
		if err != nil {
			t.Fatalf("Failed to remove existing match decision: %v", err)
		}
	}
}

// Generators

func generateCID() string {
	randBytes := make([]byte, 22)
	rand.Read(randBytes)

	// Mock CIDv0
	return fmt.Sprintf("Qm%44x", randBytes)
}

func generateEthAddress() string {
	randBytes := make([]byte, 20)
	rand.Read(randBytes)

	// Mock eth address
	return fmt.Sprintf("0x%40x", randBytes)
}

func generateEthTxHash() string {
	randBytes := make([]byte, 32)
	rand.Read(randBytes)

	// Mock eth transaction hash
	return fmt.Sprintf("0x%064x", randBytes)
}

func generateState() uint8 {
	return uint8(rand.Intn(len(data.AgreementState)))
}

func generateJobOffer() data.JobOfferContainer {
	return data.JobOfferContainer{
		ID: generateCID(),
	}
}

func generateJobOffers(min int, max int) []data.JobOfferContainer {
	count := min + rand.Intn(max-min+1)
	offers := make([]data.JobOfferContainer, count)

	for i := 0; i < count; i++ {
		offers[i] = generateJobOffer()
	}

	return offers
}

func generateResourceOffer() data.ResourceOfferContainer {
	return data.ResourceOfferContainer{
		ID:               generateCID(),
		ResourceProvider: generateEthAddress(),
	}
}

func generateResourceOffers(min int, max int) []data.ResourceOfferContainer {
	count := min + rand.Intn(max-min+1)
	offers := make([]data.ResourceOfferContainer, count)

	for i := 0; i < count; i++ {
		offers[i] = generateResourceOffer()
	}

	return offers
}

func generateDeal() data.DealContainer {
	return data.DealContainer{
		ID:               generateCID(),
		JobCreator:       generateEthAddress(),
		ResourceProvider: generateEthAddress(),
		Mediator:         generateEthAddress(),
		State:            generateState(),
		Transactions: data.DealTransactions{
			JobCreator:       data.DealTransactionsJobCreator{},
			ResourceProvider: data.DealTransactionsResourceProvider{},
			Mediator:         data.DealTransactionsMediator{},
		},
	}
}

func generateDeals(min int, max int) []data.DealContainer {
	count := min + rand.Intn(max-min+1)
	deals := make([]data.DealContainer, count)

	for i := 0; i < count; i++ {
		deals[i] = generateDeal()
	}

	return deals
}

func generateResult() data.Result {
	return data.Result{
		DealID: generateCID(),
		ID:     generateCID(),
	}
}

func generateResults(min int, max int) []data.Result {
	count := min + rand.Intn(max-min+1)
	results := make([]data.Result, count)

	for i := 0; i < count; i++ {
		results[i] = generateResult()
	}

	return results
}

func generateMatchDecision() data.MatchDecision {
	return data.MatchDecision{
		ResourceOffer: generateCID(),
		JobOffer:      generateCID(),
		Deal:          generateCID(),
		Result:        rand.Intn(2) == 1, // Generate random bool
	}
}

func generateMatchDecisions(min int, max int) []data.MatchDecision {
	count := min + rand.Intn(max-min+1)
	decisions := make([]data.MatchDecision, count)

	for i := 0; i < count; i++ {
		decisions[i] = generateMatchDecision()
	}

	return decisions
}
