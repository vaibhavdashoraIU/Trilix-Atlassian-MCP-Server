import React, { useState } from 'react';

const TrilixWorkspaces = () => {
  const [isLoggedIn, setIsLoggedIn] = useState(true);
  const [workspaces, setWorkspaces] = useState([
    {
      id: 'ws_1',
      name: 'ESO Production',
      siteUrl: 'https://eso.atlassian.net',
      email: 'developer@eso.com',
      createdAt: '2024-01-15T10:30:00Z',
    },
    {
      id: 'ws_2',
      name: 'Providentia Worldwide',
      siteUrl: 'https://providentiaww.atlassian.net',
      email: 'michal@providentia.com',
      createdAt: '2024-02-20T14:45:00Z',
    },
  ]);
  const [showModal, setShowModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [editingWorkspace, setEditingWorkspace] = useState(null);
  const [deletingWorkspace, setDeletingWorkspace] = useState(null);
  const [toast, setToast] = useState(null);
  
  const [formData, setFormData] = useState({
    name: '',
    siteUrl: '',
    email: '',
    apiToken: '',
  });

  const showToast = (message, type = 'success') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3000);
  };

  const openModal = (workspace = null) => {
    if (workspace) {
      setEditingWorkspace(workspace);
      setFormData({
        name: workspace.name,
        siteUrl: workspace.siteUrl,
        email: workspace.email,
        apiToken: '',
      });
    } else {
      setEditingWorkspace(null);
      setFormData({ name: '', siteUrl: '', email: '', apiToken: '' });
    }
    setShowModal(true);
  };

  const closeModal = () => {
    setShowModal(false);
    setEditingWorkspace(null);
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    if (editingWorkspace) {
      setWorkspaces(workspaces.map(w => 
        w.id === editingWorkspace.id 
          ? { ...w, ...formData }
          : w
      ));
      showToast('Workspace updated successfully');
    } else {
      const newWorkspace = {
        id: 'ws_' + Date.now(),
        ...formData,
        createdAt: new Date().toISOString(),
      };
      setWorkspaces([...workspaces, newWorkspace]);
      showToast('Workspace added successfully');
    }
    closeModal();
  };

  const confirmDelete = (workspace) => {
    setDeletingWorkspace(workspace);
    setShowDeleteModal(true);
  };

  const handleDelete = () => {
    setWorkspaces(workspaces.filter(w => w.id !== deletingWorkspace.id));
    setShowDeleteModal(false);
    setDeletingWorkspace(null);
    showToast('Workspace disconnected');
  };

  const formatDate = (isoString) => {
    return new Date(isoString).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  };

  const styles = {
    container: {
      minHeight: '100vh',
      background: '#1A1A2E',
      color: '#FFF8E7',
      fontFamily: "'Outfit', system-ui, sans-serif",
      position: 'relative',
      overflow: 'hidden',
    },
    starfield: {
      position: 'absolute',
      top: 0,
      left: 0,
      width: '100%',
      height: '100%',
      background: `
        radial-gradient(ellipse at 20% 20%, rgba(0, 212, 170, 0.08) 0%, transparent 50%),
        radial-gradient(ellipse at 80% 80%, rgba(255, 107, 53, 0.08) 0%, transparent 50%),
        radial-gradient(circle at 50% 50%, #1A1A2E 0%, #0D0D1A 100%)
      `,
      pointerEvents: 'none',
    },
    gridOverlay: {
      position: 'absolute',
      top: 0,
      left: 0,
      width: '100%',
      height: '100%',
      background: `
        linear-gradient(90deg, rgba(0, 212, 170, 0.03) 1px, transparent 1px),
        linear-gradient(rgba(0, 212, 170, 0.03) 1px, transparent 1px)
      `,
      backgroundSize: '60px 60px',
      pointerEvents: 'none',
    },
    content: {
      position: 'relative',
      zIndex: 10,
      maxWidth: '900px',
      margin: '0 auto',
      padding: '40px 24px',
    },
    header: {
      textAlign: 'center',
      marginBottom: '50px',
    },
    logoRing: {
      width: '120px',
      height: '120px',
      border: '3px solid #00D4AA',
      borderRadius: '50%',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      margin: '0 auto 20px',
      boxShadow: '0 0 30px rgba(0, 212, 170, 0.3)',
      position: 'relative',
    },
    logoIcon: {
      fontSize: '48px',
    },
    title: {
      fontFamily: "'Righteous', cursive",
      fontSize: 'clamp(2rem, 5vw, 3rem)',
      letterSpacing: '0.15em',
      textTransform: 'uppercase',
      background: 'linear-gradient(135deg, #FFF8E7 0%, #FF6B35 50%, #00D4AA 100%)',
      WebkitBackgroundClip: 'text',
      WebkitTextFillColor: 'transparent',
      backgroundClip: 'text',
      marginBottom: '8px',
    },
    subtitle: {
      fontFamily: "'Space Mono', monospace",
      fontSize: '0.85rem',
      color: '#00D4AA',
      letterSpacing: '0.3em',
      textTransform: 'uppercase',
    },
    authSection: {
      background: 'linear-gradient(135deg, #252542 0%, rgba(37, 37, 66, 0.8) 100%)',
      border: '2px solid #00D4AA',
      borderRadius: '20px',
      padding: '40px',
      marginBottom: '40px',
      position: 'relative',
    },
    authHeader: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      flexWrap: 'wrap',
      gap: '16px',
    },
    userInfo: {
      display: 'flex',
      alignItems: 'center',
      gap: '16px',
    },
    avatar: {
      width: '50px',
      height: '50px',
      borderRadius: '50%',
      border: '2px solid #FF6B35',
      background: 'linear-gradient(135deg, #FF6B35, #00D4AA)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: '24px',
    },
    userName: {
      fontFamily: "'Righteous', cursive",
      fontSize: '1.1rem',
      marginBottom: '2px',
    },
    userEmail: {
      fontFamily: "'Space Mono', monospace",
      fontSize: '0.75rem',
      color: '#00D4AA',
    },
    statusBadge: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: '8px',
      padding: '8px 16px',
      background: 'rgba(0, 212, 170, 0.1)',
      border: '1px solid #00D4AA',
      borderRadius: '20px',
      fontFamily: "'Space Mono', monospace",
      fontSize: '0.75rem',
      color: '#00D4AA',
      textTransform: 'uppercase',
      letterSpacing: '0.1em',
    },
    statusDot: {
      width: '8px',
      height: '8px',
      background: '#00D4AA',
      borderRadius: '50%',
      animation: 'blink 2s ease-in-out infinite',
    },
    btn: {
      fontFamily: "'Righteous', cursive",
      fontSize: '1rem',
      letterSpacing: '0.1em',
      textTransform: 'uppercase',
      padding: '14px 32px',
      border: 'none',
      borderRadius: '30px',
      cursor: 'pointer',
      transition: 'all 0.3s ease',
    },
    btnPrimary: {
      background: 'linear-gradient(135deg, #FF6B35 0%, #FF8C5A 100%)',
      color: '#1A1A2E',
      boxShadow: '0 4px 20px rgba(255, 107, 53, 0.4)',
    },
    btnSecondary: {
      background: 'transparent',
      color: '#FFF8E7',
      border: '2px solid #C0C0C0',
    },
    btnSmall: {
      padding: '10px 20px',
      fontSize: '0.85rem',
    },
    sectionHeader: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      marginBottom: '24px',
      flexWrap: 'wrap',
      gap: '16px',
    },
    sectionTitle: {
      fontFamily: "'Righteous', cursive",
      fontSize: '1.5rem',
      display: 'flex',
      alignItems: 'center',
      gap: '12px',
    },
    workspaceCard: {
      background: 'linear-gradient(135deg, #252542 0%, rgba(37, 37, 66, 0.6) 100%)',
      border: '2px solid rgba(192, 192, 192, 0.2)',
      borderRadius: '16px',
      padding: '24px',
      marginBottom: '20px',
      position: 'relative',
      transition: 'all 0.3s ease',
      borderLeft: '4px solid #FF6B35',
    },
    workspaceName: {
      fontFamily: "'Righteous', cursive",
      fontSize: '1.25rem',
      marginBottom: '4px',
    },
    workspaceUrl: {
      fontFamily: "'Space Mono', monospace",
      fontSize: '0.75rem',
      color: '#00D4AA',
    },
    workspaceMeta: {
      display: 'flex',
      gap: '20px',
      flexWrap: 'wrap',
      marginTop: '16px',
    },
    metaLabel: {
      fontFamily: "'Space Mono', monospace",
      fontSize: '0.65rem',
      color: '#C0C0C0',
      textTransform: 'uppercase',
      letterSpacing: '0.1em',
    },
    metaValue: {
      fontSize: '0.9rem',
      color: '#F5EBD6',
    },
    btnIcon: {
      width: '36px',
      height: '36px',
      borderRadius: '50%',
      border: '1px solid #C0C0C0',
      background: 'transparent',
      color: '#C0C0C0',
      cursor: 'pointer',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: '1rem',
      marginLeft: '8px',
    },
    modalOverlay: {
      position: 'fixed',
      top: 0,
      left: 0,
      width: '100%',
      height: '100%',
      background: 'rgba(13, 13, 26, 0.9)',
      backdropFilter: 'blur(8px)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      zIndex: 1000,
    },
    modal: {
      background: 'linear-gradient(135deg, #252542 0%, #1A1A2E 100%)',
      border: '2px solid #00D4AA',
      borderRadius: '24px',
      padding: '40px',
      width: '90%',
      maxWidth: '500px',
      maxHeight: '90vh',
      overflowY: 'auto',
    },
    modalTitle: {
      fontFamily: "'Righteous', cursive",
      fontSize: '1.5rem',
      marginBottom: '30px',
      display: 'flex',
      alignItems: 'center',
      gap: '12px',
    },
    formGroup: {
      marginBottom: '24px',
    },
    formLabel: {
      display: 'block',
      fontFamily: "'Space Mono', monospace",
      fontSize: '0.75rem',
      color: '#00D4AA',
      textTransform: 'uppercase',
      letterSpacing: '0.15em',
      marginBottom: '10px',
    },
    formInput: {
      width: '100%',
      padding: '14px 18px',
      background: 'rgba(0, 0, 0, 0.3)',
      border: '2px solid rgba(192, 192, 192, 0.3)',
      borderRadius: '12px',
      color: '#FFF8E7',
      fontFamily: "'Outfit', sans-serif",
      fontSize: '1rem',
      outline: 'none',
      boxSizing: 'border-box',
    },
    formHint: {
      fontFamily: "'Space Mono', monospace",
      fontSize: '0.7rem',
      color: '#C0C0C0',
      marginTop: '8px',
    },
    formActions: {
      display: 'flex',
      gap: '12px',
      marginTop: '32px',
    },
    toast: {
      position: 'fixed',
      bottom: '24px',
      right: '24px',
      padding: '16px 24px',
      background: '#252542',
      border: '2px solid #00D4AA',
      borderRadius: '12px',
      fontFamily: "'Space Mono', monospace",
      fontSize: '0.85rem',
      color: '#FFF8E7',
      display: 'flex',
      alignItems: 'center',
      gap: '12px',
      zIndex: 2000,
    },
    emptyState: {
      textAlign: 'center',
      padding: '60px 20px',
      background: 'linear-gradient(135deg, #252542 0%, rgba(37, 37, 66, 0.4) 100%)',
      border: '2px dashed rgba(192, 192, 192, 0.3)',
      borderRadius: '16px',
    },
  };

  return (
    <div style={styles.container}>
      <style>
        {`
          @import url('https://fonts.googleapis.com/css2?family=Righteous&family=Space+Mono:wght@400;700&family=Outfit:wght@300;400;500;600&display=swap');
          @keyframes blink {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.3; }
          }
        `}
      </style>
      
      <div style={styles.starfield} />
      <div style={styles.gridOverlay} />
      
      <div style={styles.content}>
        {/* Header */}
        <header style={styles.header}>
          <div style={styles.logoRing}>
            <span style={styles.logoIcon}>🚀</span>
          </div>
          <h1 style={styles.title}>Trilix Command Center</h1>
          <p style={styles.subtitle}>Atlassian Workspace Control</p>
        </header>

        {/* Auth Section */}
        <section style={styles.authSection}>
          {!isLoggedIn ? (
            <div style={{ textAlign: 'center' }}>
              <h2 style={{ fontFamily: "'Righteous', cursive", fontSize: '1.8rem', marginBottom: '12px' }}>
                Welcome, Space Cadet!
              </h2>
              <p style={{ color: '#C0C0C0', marginBottom: '30px' }}>
                Sign in to connect your Atlassian workspaces and launch your AI-powered mission control.
              </p>
              <button 
                style={{ ...styles.btn, ...styles.btnPrimary }}
                onClick={() => setIsLoggedIn(true)}
              >
                🔐 Initialize Login Sequence
              </button>
            </div>
          ) : (
            <div style={styles.authHeader}>
              <div style={styles.userInfo}>
                <div style={styles.avatar}>👨‍🚀</div>
                <div>
                  <h3 style={styles.userName}>Michal</h3>
                  <p style={styles.userEmail}>michal@providentia.com</p>
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                <div style={styles.statusBadge}>
                  <span style={styles.statusDot} />
                  Systems Online
                </div>
                <button 
                  style={{ ...styles.btn, ...styles.btnSecondary, ...styles.btnSmall }}
                  onClick={() => setIsLoggedIn(false)}
                >
                  Sign Out
                </button>
              </div>
            </div>
          )}
        </section>

        {/* Workspaces Section */}
        {isLoggedIn && (
          <section>
            <div style={styles.sectionHeader}>
              <h2 style={styles.sectionTitle}>
                <span style={{ color: '#FF6B35' }}>◈</span> Connected Workspaces
              </h2>
              <button 
                style={{ ...styles.btn, ...styles.btnPrimary, ...styles.btnSmall }}
                onClick={() => openModal()}
              >
                + Add Workspace
              </button>
            </div>

            {workspaces.length === 0 ? (
              <div style={styles.emptyState}>
                <div style={{ fontSize: '4rem', marginBottom: '20px', opacity: 0.5 }}>🛸</div>
                <h3 style={{ fontFamily: "'Righteous', cursive", fontSize: '1.3rem', marginBottom: '8px' }}>
                  No Workspaces Detected
                </h3>
                <p style={{ color: '#C0C0C0' }}>
                  Add your first Atlassian workspace to begin your mission.
                </p>
              </div>
            ) : (
              workspaces.map(workspace => (
                <div key={workspace.id} style={styles.workspaceCard}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                    <div>
                      <h3 style={styles.workspaceName}>{workspace.name}</h3>
                      <p style={styles.workspaceUrl}>{workspace.siteUrl}</p>
                    </div>
                    <div style={{ display: 'flex' }}>
                      <button 
                        style={styles.btnIcon}
                        onClick={() => openModal(workspace)}
                      >
                        ✎
                      </button>
                      <button 
                        style={{ ...styles.btnIcon, borderColor: '#FF4757', color: '#FF4757' }}
                        onClick={() => confirmDelete(workspace)}
                      >
                        🗑
                      </button>
                    </div>
                  </div>
                  <div style={styles.workspaceMeta}>
                    <div>
                      <div style={styles.metaLabel}>Email</div>
                      <div style={styles.metaValue}>{workspace.email}</div>
                    </div>
                    <div>
                      <div style={styles.metaLabel}>API Token</div>
                      <div style={{ ...styles.metaValue, fontFamily: "'Space Mono', monospace", letterSpacing: '0.2em' }}>
                        ••••••••••••
                      </div>
                    </div>
                    <div>
                      <div style={styles.metaLabel}>Connected</div>
                      <div style={styles.metaValue}>{formatDate(workspace.createdAt)}</div>
                    </div>
                  </div>
                </div>
              ))
            )}
          </section>
        )}
      </div>

      {/* Add/Edit Modal */}
      {showModal && (
        <div style={styles.modalOverlay} onClick={(e) => e.target === e.currentTarget && closeModal()}>
          <div style={styles.modal}>
            <h3 style={styles.modalTitle}>
              <span style={{ color: '#FF6B35' }}>⬡</span>
              {editingWorkspace ? 'Edit Workspace' : 'Add Workspace'}
            </h3>
            <form onSubmit={handleSubmit}>
              <div style={styles.formGroup}>
                <label style={styles.formLabel}>Workspace Name</label>
                <input
                  type="text"
                  style={styles.formInput}
                  placeholder="e.g., ESO Production"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  required
                />
                <p style={styles.formHint}>A friendly name to identify this workspace</p>
              </div>
              <div style={styles.formGroup}>
                <label style={styles.formLabel}>Site URL</label>
                <input
                  type="url"
                  style={styles.formInput}
                  placeholder="https://your-org.atlassian.net"
                  value={formData.siteUrl}
                  onChange={(e) => setFormData({ ...formData, siteUrl: e.target.value })}
                  required
                />
                <p style={styles.formHint}>Your Atlassian instance URL (without /wiki)</p>
              </div>
              <div style={styles.formGroup}>
                <label style={styles.formLabel}>Email Address</label>
                <input
                  type="email"
                  style={styles.formInput}
                  placeholder="your-email@company.com"
                  value={formData.email}
                  onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                  required
                />
                <p style={styles.formHint}>The email associated with your Atlassian account</p>
              </div>
              <div style={styles.formGroup}>
                <label style={styles.formLabel}>API Token</label>
                <input
                  type="password"
                  style={styles.formInput}
                  placeholder={editingWorkspace ? "(unchanged)" : "ATATT3xFfGF0..."}
                  value={formData.apiToken}
                  onChange={(e) => setFormData({ ...formData, apiToken: e.target.value })}
                  required={!editingWorkspace}
                />
                <p style={styles.formHint}>
                  Create at{' '}
                  <a href="https://id.atlassian.com/manage-profile/security/api-tokens" target="_blank" style={{ color: '#00D4AA' }}>
                    id.atlassian.com
                  </a>
                </p>
              </div>
              <div style={styles.formActions}>
                <button 
                  type="button" 
                  style={{ ...styles.btn, ...styles.btnSecondary, flex: 1 }}
                  onClick={closeModal}
                >
                  Cancel
                </button>
                <button 
                  type="submit" 
                  style={{ ...styles.btn, ...styles.btnPrimary, flex: 1 }}
                >
                  {editingWorkspace ? '💾 Save Changes' : '🚀 Launch'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {showDeleteModal && (
        <div style={styles.modalOverlay} onClick={(e) => e.target === e.currentTarget && setShowDeleteModal(false)}>
          <div style={{ ...styles.modal, maxWidth: '400px', textAlign: 'center' }}>
            <h3 style={{ ...styles.modalTitle, justifyContent: 'center' }}>Confirm Deletion</h3>
            <p style={{ color: '#C0C0C0', marginBottom: '24px' }}>
              Are you sure you want to disconnect{' '}
              <strong style={{ color: '#FF6B35' }}>{deletingWorkspace?.name}</strong>?
            </p>
            <p style={{ color: '#FF4757', fontSize: '0.85rem', marginBottom: '24px' }}>
              This action cannot be undone.
            </p>
            <div style={styles.formActions}>
              <button 
                type="button" 
                style={{ ...styles.btn, ...styles.btnSecondary, flex: 1 }}
                onClick={() => setShowDeleteModal(false)}
              >
                Cancel
              </button>
              <button 
                type="button" 
                style={{ ...styles.btn, flex: 1, background: 'transparent', border: '2px solid #FF4757', color: '#FF4757' }}
                onClick={handleDelete}
              >
                🗑️ Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Toast */}
      {toast && (
        <div style={{ ...styles.toast, borderColor: toast.type === 'error' ? '#FF4757' : '#00D4AA' }}>
          <span>{toast.type === 'success' ? '✓' : '⚠'}</span>
          <span>{toast.message}</span>
        </div>
      )}
    </div>
  );
};

export default TrilixWorkspaces;
