import { useEffect, useState } from "react";

import { defaultDestination, destinationFromHash, destinationHash, type ConsoleDestination } from "console/routes";

export function useConsoleRouter() {
  const [destination, setDestination] = useState<ConsoleDestination>(() =>
    typeof window === "undefined" ? defaultDestination : destinationFromHash(window.location.hash)
  );

  useEffect(() => {
    function syncDestinationFromHash() {
      setDestination(destinationFromHash(window.location.hash));
    }

    window.addEventListener("hashchange", syncDestinationFromHash);
    return () => window.removeEventListener("hashchange", syncDestinationFromHash);
  }, []);

  function navigate(next: ConsoleDestination) {
    setDestination(next);
    const nextHash = destinationHash(next);
    if (window.location.hash !== nextHash) {
      window.location.hash = nextHash;
    }
  }

  return { destination, navigate };
}
