import { useQuery } from "@tanstack/react-query";

import { getActiveConfig, getHealth } from "platform/api/configApi";
import { getTrafficSpoolStatus } from "platform/api/spool";
import { useI18n } from "platform/i18n";
import { queryKeys } from "platform/query/keys";
import type { PageHeading } from "shared/ui/PageHeading";
import { PolicyActivationPanel } from "../ui/PolicyActivationPanel";
import { PolicyAuthoringPanel } from "../ui/PolicyAuthoringPanel";
import { PolicyBundlesTable } from "../ui/PolicyBundlesTable";
import { listPolicyBundles } from "../model/policiesApi";
import { RegexLabPanel } from "../ui/RegexLabPanel";
import { usePolicyWorkflow } from "../model/usePolicyWorkflow";

export type PoliciesPageProps = {
  heading: PageHeading;
  tab: PoliciesTab;
};

export type PoliciesTab = "rule-library" | "activation";

export function PoliciesPage({ heading, tab }: PoliciesPageProps) {
  const { t } = useI18n();
  const activeConfig = useQuery({
    queryKey: queryKeys.activeConfig,
    queryFn: getActiveConfig,
    retry: 1
  });
  const health = useQuery({
    queryKey: queryKeys.health,
    queryFn: getHealth,
    retry: 1
  });
  const policies = useQuery({
    queryKey: queryKeys.policyBundles,
    queryFn: listPolicyBundles,
    retry: 1
  });
  const spoolStatus = useQuery({
    queryKey: queryKeys.trafficSpool,
    queryFn: getTrafficSpoolStatus,
    retry: 1,
    refetchInterval: 5000
  });
  const policyItems = policies.data?.items ?? [];
  const policyWorkflow = usePolicyWorkflow(policyItems, t);
  const isActivation = tab === "activation";

  return (
    <>
      {isActivation ? (
        <PolicyActivationPanel
          activeKeys={activeConfig.data?.active_policy_bundle_keys ?? []}
          activationMessage={policyWorkflow.activationMessage}
          activationMessageKind={policyWorkflow.activationMessageKind}
          activationReason={policyWorkflow.activationReason}
          heading={heading}
          isActivating={policyWorkflow.activateSelectedPolicies.isPending}
          isSpoolError={spoolStatus.isError}
          isSpoolLoading={spoolStatus.isLoading}
          isValidating={policyWorkflow.validateSelectedPolicies.isPending}
          policyItems={policyItems}
          selectedPolicyKeys={policyWorkflow.selectedPolicyKeys}
          snapshotHash={activeConfig.data?.policy_snapshot_hash || t("common.none")}
          spool={spoolStatus.data}
          spoolTitle={t("runtime.spoolStatus")}
          version={health.data?.version ?? "unknown"}
          onActivate={policyWorkflow.activateSelected}
          onReasonChange={policyWorkflow.setActivationReason}
          onSelectionChange={policyWorkflow.togglePolicySelection}
          onValidate={policyWorkflow.validateSelected}
        />
      ) : (
        <PolicyBundlesTable heading={heading} policyItems={policyItems} />
      )}
      {!isActivation ? (
        <PolicyAuthoringPanel
          isSaving={policyWorkflow.savePolicy.isPending}
          isValidating={policyWorkflow.validatePolicy.isPending}
          message={policyWorkflow.policyEditorMessage}
          messageKind={policyWorkflow.policyEditorMessageKind}
          policyBundleJSON={policyWorkflow.policyBundleJSON}
          onChange={policyWorkflow.setPolicyBundleJSON}
          onSave={policyWorkflow.savePolicyEditor}
          onValidate={policyWorkflow.validatePolicyEditor}
        />
      ) : null}
      {!isActivation ? (
        <RegexLabPanel
          isError={policyWorkflow.regex.isError}
          isPending={policyWorkflow.regex.isPending}
          isSuccess={policyWorkflow.regex.isSuccess}
          matches={policyWorkflow.matches}
          pattern={policyWorkflow.pattern}
          sampleText={policyWorkflow.sampleText}
          onPatternChange={policyWorkflow.setPattern}
          onRun={policyWorkflow.runRegex}
          onSampleTextChange={policyWorkflow.setSampleText}
        />
      ) : null}
    </>
  );
}
