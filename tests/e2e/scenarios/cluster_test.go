package scenarios

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"nebulagc.io/tests/e2e/fixtures"
	"nebulagc.io/tests/e2e/helpers"
)

// Test database fixture creation and basic queries
func TestClusterFixtures(t *testing.T) {
	// Setup
	db := helpers.NewTestDB(t)

	t.Run("CreateTenant", func(t *testing.T) {
		tenantID := fixtures.Tenant(t, db.DB, "test-tenant")

		// Verify tenant created
		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM tenants WHERE id = ?", tenantID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify UUID format
		_, err = uuid.Parse(tenantID)
		require.NoError(t, err, "tenant ID should be valid UUID")
	})

	t.Run("CreateCluster", func(t *testing.T) {
		tenantID := fixtures.Tenant(t, db.DB, "cluster-tenant")
		clusterID, clusterToken := fixtures.Cluster(t, db.DB, tenantID, "test-cluster", fixtures.TestHMACSecret)

		// Verify cluster created
		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM clusters WHERE id = ?", clusterID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify token generated
		assert.NotEmpty(t, clusterToken)
		assert.GreaterOrEqual(t, len(clusterToken), 41, "token should be at least 41 characters")
	})

	t.Run("CreateMultipleClusters", func(t *testing.T) {
		tenantID := fixtures.Tenant(t, db.DB, "multi-cluster-tenant")

		// Create 5 clusters
		clusterIDs := make([]string, 5)
		for i := 0; i < 5; i++ {
			clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, fixtures.RandomName("cluster"), fixtures.TestHMACSecret)
			clusterIDs[i] = clusterID
		}

		// Verify all created
		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM clusters WHERE tenant_id = ?", tenantID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 5, count)

		// Verify all UUIDs are unique
		uniqueMap := make(map[string]bool)
		for _, id := range clusterIDs {
			uniqueMap[id] = true
		}
		assert.Equal(t, 5, len(uniqueMap), "all cluster IDs should be unique")
	})

	t.Run("DeleteClusterCascade", func(t *testing.T) {
		helpers.CleanupTables(t, db.DB)

		tenantID := fixtures.Tenant(t, db.DB, "cascade-tenant")
		clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, "cascade-cluster", fixtures.TestHMACSecret)

		// Add nodes to cluster
		fixtures.Node(t, db.DB, tenantID, clusterID, "node1", fixtures.TestHMACSecret, false)
		fixtures.Node(t, db.DB, tenantID, clusterID, "node2", fixtures.TestHMACSecret, false)

		// Verify nodes exist
		var nodeCount int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE cluster_id = ?", clusterID).Scan(&nodeCount)
		require.NoError(t, err)
		assert.Equal(t, 2, nodeCount)

		// Delete cluster
		helpers.MustExec(t, db.DB, "DELETE FROM clusters WHERE id = ?", clusterID)

		// Verify cascade delete of nodes
		err = db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE cluster_id = ?", clusterID).Scan(&nodeCount)
		require.NoError(t, err)
		assert.Equal(t, 0, nodeCount, "nodes should be cascade deleted")
	})
}

func TestClusterQueries(t *testing.T) {
	// Setup
	db := helpers.NewTestDB(t)

	// Setup test data
	tenantID := fixtures.Tenant(t, db.DB, "query-tenant")
	clusterID1, token1 := fixtures.Cluster(t, db.DB, tenantID, "cluster-1", fixtures.TestHMACSecret)
	clusterID2, token2 := fixtures.Cluster(t, db.DB, tenantID, "cluster-2", fixtures.TestHMACSecret)
	_ = clusterID2 // Used in TokenUniqueness test

	t.Run("QueryClusterByID", func(t *testing.T) {
		var cluster struct {
			ID             string
			TenantID       string
			Name           string
			LighthousePort int
			ConfigVersion  int64
		}

		err := db.DB.QueryRow(`
			SELECT id, tenant_id, name, lighthouse_port, config_version
			FROM clusters 
			WHERE id = ?
		`, clusterID1).Scan(
			&cluster.ID,
			&cluster.TenantID,
			&cluster.Name,
			&cluster.LighthousePort,
			&cluster.ConfigVersion,
		)

		require.NoError(t, err)
		assert.Equal(t, clusterID1, cluster.ID)
		assert.Equal(t, tenantID, cluster.TenantID)
		assert.Equal(t, "cluster-1", cluster.Name)
		assert.Equal(t, 4242, cluster.LighthousePort)
		assert.Equal(t, int64(0), cluster.ConfigVersion)
	})

	t.Run("QueryAllClustersForTenant", func(t *testing.T) {
		rows := helpers.MustQuery(t, db.DB, `
			SELECT id, name
			FROM clusters 
			WHERE tenant_id = ?
			ORDER BY name
		`, tenantID)
		defer rows.Close()

		clusters := make([]struct {
			ID   string
			Name string
		}, 0)

		for rows.Next() {
			var c struct {
				ID   string
				Name string
			}
			require.NoError(t, rows.Scan(&c.ID, &c.Name))
			clusters = append(clusters, c)
		}

		require.Equal(t, 2, len(clusters))
		assert.Equal(t, "cluster-1", clusters[0].Name)
		assert.Equal(t, "cluster-2", clusters[1].Name)
	})

	t.Run("UpdateConfigVersion", func(t *testing.T) {
		// Update version
		helpers.MustExec(t, db.DB, `
			UPDATE clusters 
			SET config_version = config_version + 1 
			WHERE id = ?
		`, clusterID1)

		// Verify update
		var version int64
		err := db.DB.QueryRow("SELECT config_version FROM clusters WHERE id = ?", clusterID1).Scan(&version)
		require.NoError(t, err)
		assert.Equal(t, int64(1), version)
	})

	t.Run("TokenUniqueness", func(t *testing.T) {
		// Verify tokens are different
		assert.NotEqual(t, token1, token2, "cluster tokens should be unique")

		// Verify token hashes are in database
		var count int
		err := db.DB.QueryRow("SELECT COUNT(DISTINCT cluster_token_hash) FROM clusters WHERE tenant_id = ?", tenantID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count, "should have 2 unique token hashes")
	})
}

func TestNodeDatabase(t *testing.T) {
	// Setup
	db := helpers.NewTestDB(t)

	// Setup test data
	tenantID := fixtures.Tenant(t, db.DB, "node-tenant")
	clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, "node-cluster", fixtures.TestHMACSecret)

	t.Run("CreateNode", func(t *testing.T) {
		nodeID, _ := fixtures.Node(t, db.DB, tenantID, clusterID, "test-node", fixtures.TestHMACSecret, false)

		// Verify node created
		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE id = ?", nodeID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("CreateAdminNode", func(t *testing.T) {
		nodeID, _ := fixtures.AdminNode(t, db.DB, tenantID, clusterID, "admin-node", fixtures.TestHMACSecret)

		// Verify admin flag in database
		var isAdmin bool
		err := db.DB.QueryRow("SELECT is_admin FROM nodes WHERE id = ?", nodeID).Scan(&isAdmin)
		require.NoError(t, err)
		assert.True(t, isAdmin)
	})

	t.Run("CreateLighthouseNode", func(t *testing.T) {
		nodeID, _ := fixtures.LighthouseNode(t, db.DB, tenantID, clusterID, "lighthouse-1", fixtures.TestHMACSecret, "1.2.3.4", 4242)

		// Verify lighthouse properties
		var isLighthouse bool
		var publicIP string
		var publicPort int
		err := db.DB.QueryRow(`
			SELECT is_lighthouse, lighthouse_public_ip, lighthouse_public_port 
			FROM nodes 
			WHERE id = ?
		`, nodeID).Scan(&isLighthouse, &publicIP, &publicPort)
		require.NoError(t, err)
		assert.True(t, isLighthouse)
		assert.Equal(t, "1.2.3.4", publicIP)
		assert.Equal(t, 4242, publicPort)
	})

	t.Run("ListNodes", func(t *testing.T) {
		// Clean and create fresh data
		helpers.CleanupTables(t, db.DB)
		tenantID := fixtures.Tenant(t, db.DB, "list-tenant")
		clusterID, _ := fixtures.Cluster(t, db.DB, tenantID, "list-cluster", fixtures.TestHMACSecret)

		// Create nodes
		for i := 1; i <= 3; i++ {
			name := fixtures.RandomName("node")
			isAdmin := i == 1
			if isAdmin {
				fixtures.AdminNode(t, db.DB, tenantID, clusterID, name, fixtures.TestHMACSecret)
			} else {
				fixtures.Node(t, db.DB, tenantID, clusterID, name, fixtures.TestHMACSecret, false)
			}
		}

		// Query nodes
		var count int
		err := db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE cluster_id = ?", clusterID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Verify admin count
		var adminCount int
		err = db.DB.QueryRow("SELECT COUNT(*) FROM nodes WHERE cluster_id = ? AND is_admin = 1", clusterID).Scan(&adminCount)
		require.NoError(t, err)
		assert.Equal(t, 1, adminCount)
	})
}
