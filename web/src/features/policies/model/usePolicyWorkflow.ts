import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FormEvent, useEffect, useRef, useState } from "react";

import { activatePolicyBundles } from "platform/api/policyActivation";
import { queryKeys } from "platform/query/keys";
import { shortHash } from "shared/ui/format";
import type { MessageKind } from "shared/ui/status";
import { createPolicyBundle, testRegex, validatePolicyBundle, type PolicyBundle } from "./policiesApi";
import { starterPolicyBundleJSON } from "./starterPolicyBundle";

type Translator = (key: string, values?: Record<string, string | number>) => string;

export function usePolicyWorkflow(policyItems: PolicyBundle[], t: Translator) {
  const queryClient = useQueryClient();
  const defaultPattern = t("regex.defaultPattern");
  const defaultSampleText = t("regex.defaultSampleText");
  const localizedDefaultsRef = useRef({ pattern: defaultPattern, sampleText: defaultSampleText });
  const [pattern, setPattern] = useState(defaultPattern);
  const [sampleText, setSampleText] = useState(defaultSampleText);
  const [policyBundleJSON, setPolicyBundleJSON] = useState(starterPolicyBundleJSON);
  const [policyEditorMessage, setPolicyEditorMessage] = useState("");
  const [policyEditorMessageKind, setPolicyEditorMessageKind] = useState<MessageKind>("success");
  const [selectedPolicyKeys, setSelectedPolicyKeys] = useState<string[]>([]);
  const [activationReason, setActivationReason] = useState("");
  const [activationMessage, setActivationMessage] = useState("");
  const [activationMessageKind, setActivationMessageKind] = useState<MessageKind>("success");

  useEffect(() => {
    const previousDefaults = localizedDefaultsRef.current;

    setPattern((currentPattern) => (currentPattern === previousDefaults.pattern ? defaultPattern : currentPattern));
    setSampleText((currentSampleText) => (currentSampleText === previousDefaults.sampleText ? defaultSampleText : currentSampleText));

    localizedDefaultsRef.current = { pattern: defaultPattern, sampleText: defaultSampleText };
  }, [defaultPattern, defaultSampleText]);

  const regex = useMutation({ mutationFn: testRegex });
  const validatePolicy = useMutation({
    mutationFn: (bundle: PolicyBundle) => validatePolicyBundle(bundle),
    onSuccess: (response) => {
      setPolicyEditorMessageKind(response.valid ? "success" : "error");
      setPolicyEditorMessage(response.valid ? t("authoring.valid", { hash: response.hash ? t("authoring.hashSuffix", { hash: shortHash(response.hash) }) : "" }) : response.error || t("authoring.invalid"));
    },
    onError: (error) => {
      setPolicyEditorMessageKind("error");
      setPolicyEditorMessage(error instanceof Error ? error.message : t("authoring.validationFailed"));
    }
  });
  const savePolicy = useMutation({
    mutationFn: (bundle: PolicyBundle) => createPolicyBundle(bundle),
    onSuccess: async (bundle) => {
      setPolicyEditorMessageKind("success");
      setPolicyEditorMessage(t("authoring.saved", { key: bundle.key }));
      await queryClient.invalidateQueries({ queryKey: queryKeys.policyBundles });
    },
    onError: (error) => {
      setPolicyEditorMessageKind("error");
      setPolicyEditorMessage(error instanceof Error ? error.message : t("authoring.saveFailed"));
    }
  });
  const validateSelectedPolicies = useMutation({
    mutationFn: async (bundles: PolicyBundle[]) => Promise.all(bundles.map((bundle) => validatePolicyBundle(bundle))),
    onSuccess: (responses) => {
      const invalid = responses.find((response) => !response.valid);
      setActivationMessageKind(invalid ? "error" : "success");
      setActivationMessage(invalid ? invalid.error || t("runtime.validationFailed") : t("runtime.validated", { count: responses.length, plural: responses.length === 1 ? "" : "s" }));
    },
    onError: (error) => {
      setActivationMessageKind("error");
      setActivationMessage(error instanceof Error ? error.message : t("runtime.validationFailed"));
    }
  });
  const activateSelectedPolicies = useMutation({
    mutationFn: () => activatePolicyBundles({ keys: selectedPolicyKeys, reason: activationReason.trim() || undefined }),
    onSuccess: async (response) => {
      const revision = response.revision_number ? t("runtime.revisionSuffix", { revision: response.revision_number }) : "";
      const notification = response.notification_error ? t("runtime.notificationSuffix", { notification: response.notification_error }) : "";
      setActivationMessageKind(response.notification_error ? "error" : "success");
      setActivationMessage(t("runtime.activated", { count: response.active_keys.length, plural: response.active_keys.length === 1 ? "" : "s", revision, notification }));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.activeConfig }),
        queryClient.invalidateQueries({ queryKey: queryKeys.consoleSummary }),
        queryClient.invalidateQueries({ queryKey: queryKeys.modelMappings }),
        queryClient.invalidateQueries({ queryKey: queryKeys.policyBundles }),
        queryClient.invalidateQueries({ queryKey: queryKeys.verdictProviders })
      ]);
    },
    onError: (error) => {
      setActivationMessageKind("error");
      setActivationMessage(error instanceof Error ? error.message : t("runtime.validationFailed"));
    }
  });

  function runRegex(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    regex.mutate({ pattern, text: sampleText });
  }

  function parsePolicyEditorBundle() {
    try {
      return JSON.parse(policyBundleJSON) as PolicyBundle;
    } catch {
      setPolicyEditorMessageKind("error");
      setPolicyEditorMessage(t("authoring.invalidJson"));
      return null;
    }
  }

  function validatePolicyEditor(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const bundle = parsePolicyEditorBundle();
    if (bundle) validatePolicy.mutate(bundle);
  }

  function savePolicyEditor() {
    const bundle = parsePolicyEditorBundle();
    if (bundle) savePolicy.mutate(bundle);
  }

  function togglePolicySelection(key: string, checked: boolean) {
    setSelectedPolicyKeys((keys) => (checked ? [...new Set([...keys, key])] : keys.filter((item) => item !== key)));
  }

  function selectedPolicyBundles() {
    return selectedPolicyKeys.map((key) => policyItems.find((bundle) => bundle.key === key)).filter((bundle): bundle is PolicyBundle => Boolean(bundle));
  }

  function validateSelected() {
    const bundles = selectedPolicyBundles();
    if (bundles.length === 0) {
      setActivationMessageKind("error");
      setActivationMessage(t("runtime.selectValidate"));
      return;
    }
    validateSelectedPolicies.mutate(bundles);
  }

  function activateSelected() {
    if (selectedPolicyKeys.length === 0) {
      setActivationMessageKind("error");
      setActivationMessage(t("runtime.selectActivate"));
      return;
    }
    activateSelectedPolicies.mutate();
  }

  return {
    activationMessage,
    activationMessageKind,
    activationReason,
    matches: regex.data?.matches ?? [],
    pattern,
    policyBundleJSON,
    policyEditorMessage,
    policyEditorMessageKind,
    regex,
    sampleText,
    savePolicy,
    selectedPolicyKeys,
    validatePolicy,
    validateSelectedPolicies,
    activateSelectedPolicies,
    activateSelected,
    runRegex,
    savePolicyEditor,
    setActivationReason,
    setPattern,
    setPolicyBundleJSON,
    setSampleText,
    togglePolicySelection,
    validatePolicyEditor,
    validateSelected
  };
}
