import { Navigate, Route, Routes, useParams } from "react-router-dom";
import { lazyNamed } from "./lazyNamed";

const FishSettingsPage = lazyNamed(
  () => import("../ui-v2/fish/FishSettingsPage"),
  "FishSettingsPage",
);
const BackupPage = lazyNamed(
  () => import("../modules/backup/BackupPage"),
  "BackupPage",
);
const S3ConfigPage = lazyNamed(
  () => import("../modules/backup/channels/s3/S3ConfigPage"),
  "S3ConfigPage",
);
const FishProfilePage = lazyNamed(
  () => import("../ui-v2/fish/FishProfilePage"),
  "FishProfilePage",
);
const NotificationsPage = lazyNamed(
  () => import("../modules/notifications/NotificationsPage"),
  "NotificationsPage",
);
const FishChartsPage = lazyNamed(
  () => import("../ui-v2/fish/FishChartsPage"),
  "FishChartsPage",
);
const FishBrowserListPage = lazyNamed(
  () => import("../ui-v2/fish/FishBrowserListPage"),
  "FishBrowserListPage",
);
const FishBrowserDetailPage = lazyNamed(
  () => import("../ui-v2/fish/FishBrowserDetailPage"),
  "FishBrowserDetailPage",
);
const FishBrowserEditPage = lazyNamed(
  () => import("../ui-v2/fish/FishBrowserEditPage"),
  "FishBrowserEditPage",
);
const FishBrowserCopyPage = lazyNamed(
  () => import("../ui-v2/fish/FishBrowserCopyPage"),
  "FishBrowserCopyPage",
);
const FishLogsPage = lazyNamed(
  () => import("../ui-v2/fish/FishLogsPage"),
  "FishLogsPage",
);
const FishProxyPoolPage = lazyNamed(
  () => import("../ui-v2/fish/FishProxyPoolPage"),
  "FishProxyPoolPage",
);
const FishCoreManagementPage = lazyNamed(
  () => import("../ui-v2/fish/FishCoreManagementPage"),
  "FishCoreManagementPage",
);
const FishBookmarksPage = lazyNamed(
  () => import("../ui-v2/fish/FishBookmarksPage"),
  "FishBookmarksPage",
);
const FishExtensionsPage = lazyNamed(
  () => import("../ui-v2/fish/FishExtensionsPage"),
  "FishExtensionsPage",
);
const FishDocsPage = lazyNamed(
  () => import("../ui-v2/fish/FishDocsPage"),
  "FishDocsPage",
);
const FishTagsPage = lazyNamed(
  () => import("../ui-v2/fish/FishTagsPage"),
  "FishTagsPage",
);
const FishAutomationPage = lazyNamed(
  () => import("../ui-v2/fish/FishAutomationPage"),
  "FishAutomationPage",
);
const FishAutomationDetailPage = lazyNamed(
  () => import("../ui-v2/fish/FishAutomationDetailPage"),
  "FishAutomationDetailPage",
);
const FishWorkflowsPage = lazyNamed(
  () => import("../ui-v2/fish/FishWorkflowsPage"),
  "FishWorkflowsPage",
);
const FishControlCenterPage = lazyNamed(
  () => import("../ui-v2/fish/FishControlCenterPage"),
  "FishControlCenterPage",
);

function LegacyBrowserProfileRedirect({ action }: { action: "detail" | "edit" | "copy" }) {
  const { id } = useParams();
  return <Navigate to={id ? `/browser/${action}/${id}` : "/browser/list"} replace />;
}

function LegacyAutomationDetailRedirect() {
  const { scriptId } = useParams();
  return <Navigate to={scriptId ? `/browser/automation/${scriptId}` : "/browser/automation"} replace />;
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/browser/list" replace />} />

      <Route path="/charts" element={<FishChartsPage />} />
      <Route path="/charts-legacy" element={<Navigate to="/charts" replace />} />

      <Route path="/settings" element={<FishSettingsPage />} />
      <Route path="/settings-legacy" element={<Navigate to="/settings" replace />} />
      <Route path="/system/backup" element={<BackupPage />} />
      <Route path="/system/backup/s3" element={<S3ConfigPage />} />

      <Route path="/profile" element={<FishProfilePage />} />
      <Route path="/profile-legacy" element={<Navigate to="/profile" replace />} />
      <Route path="/notifications" element={<NotificationsPage />} />

      <Route path="/browser/list" element={<FishBrowserListPage />} />
      <Route path="/browser/list-v2" element={<Navigate to="/browser/list" replace />} />
      <Route path="/browser/list-legacy" element={<Navigate to="/browser/list" replace />} />
      <Route path="/browser/monitor" element={<Navigate to="/browser/list" replace />} />

      <Route path="/browser/detail/:id" element={<FishBrowserDetailPage />} />
      <Route path="/browser/detail-legacy/:id" element={<LegacyBrowserProfileRedirect action="detail" />} />

      <Route path="/browser/edit/:id" element={<FishBrowserEditPage />} />
      <Route path="/browser/edit-fish/:id" element={<LegacyBrowserProfileRedirect action="edit" />} />
      <Route path="/browser/edit-legacy/:id" element={<LegacyBrowserProfileRedirect action="edit" />} />

      <Route path="/browser/copy/:id" element={<FishBrowserCopyPage />} />
      <Route path="/browser/copy-legacy/:id" element={<LegacyBrowserProfileRedirect action="copy" />} />

      <Route path="/browser/logs" element={<FishLogsPage />} />
      <Route path="/browser/logs-legacy" element={<Navigate to="/browser/logs" replace />} />

      <Route path="/browser/proxy-pool" element={<FishProxyPoolPage />} />
      <Route path="/browser/proxy-pool-legacy" element={<Navigate to="/browser/proxy-pool" replace />} />

      <Route path="/browser/cores" element={<FishCoreManagementPage />} />
      <Route path="/browser/cores-legacy" element={<Navigate to="/browser/cores" replace />} />

      <Route path="/browser/extensions" element={<FishExtensionsPage />} />
      <Route path="/browser/extensions-legacy" element={<Navigate to="/browser/extensions" replace />} />

      <Route path="/browser/bookmarks" element={<FishBookmarksPage />} />
      <Route path="/browser/bookmarks-legacy" element={<Navigate to="/browser/bookmarks" replace />} />

      <Route path="/browser/automation" element={<FishAutomationPage />} />
      <Route path="/browser/automation-legacy" element={<Navigate to="/browser/automation" replace />} />
      <Route path="/browser/automation/:scriptId" element={<FishAutomationDetailPage />} />
      <Route path="/browser/automation-legacy/:scriptId" element={<LegacyAutomationDetailRedirect />} />
      <Route path="/browser/workflows" element={<FishWorkflowsPage />} />

      <Route path="/system/control" element={<FishControlCenterPage />} />
      <Route path="/system/docs" element={<FishDocsPage />} />
      <Route path="/system/docs-legacy" element={<Navigate to="/system/docs" replace />} />
      <Route path="/browser/launch-api" element={<Navigate to="/system/docs" replace />} />
      <Route path="/system/tutorial" element={<Navigate to="/system/docs" replace />} />

      <Route path="/browser/tags" element={<FishTagsPage />} />
      <Route path="/browser/tags-legacy" element={<Navigate to="/browser/tags" replace />} />
    </Routes>
  );
}
