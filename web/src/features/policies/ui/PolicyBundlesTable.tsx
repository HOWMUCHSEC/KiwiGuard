import {
  DataTable,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableHeader,
  TableRow,
  Tile
} from "@carbon/react";
import { RefreshCcw } from "lucide-react";

import type { PolicyBundle } from "../model/policiesApi";
import { useI18n } from "platform/i18n";
import { PanelHeader } from "shared/ui/PanelHeader";
import type { PageHeading } from "shared/ui/PageHeading";

export function PolicyBundlesTable({ heading, policyItems }: { heading: PageHeading; policyItems: PolicyBundle[] }) {
  const { t } = useI18n();
  const headers = [
    { key: "key", header: t("policy.key") },
    { key: "version", header: t("policy.version") },
    { key: "source", header: t("policy.source") },
    { key: "defaultAction", header: t("policy.default") },
    { key: "rules", header: t("policy.rules") }
  ];
  const rows = policyItems.map((bundle) => ({
    id: bundle.key,
    key: bundle.key,
    version: bundle.version,
    source: bundle.source,
    defaultAction: bundle.default_action,
    rules: String(bundle.rules?.length ?? 0)
  }));

  return (
    <Tile className="kg-panel span-2">
      <PanelHeader icon={<RefreshCcw aria-hidden="true" />} kicker={heading.kicker} title={heading.title} />
      <DataTable rows={rows} headers={headers}>
        {({ rows, headers, getHeaderProps, getRowProps, getTableProps }) => (
          <TableContainer>
            <Table {...getTableProps()} useZebraStyles>
              <TableHead>
                <TableRow>
                  {headers.map((header) => (
                    <TableHeader {...getHeaderProps({ header })} key={header.key}>
                      {header.header}
                    </TableHeader>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {rows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={headers.length}>{t("policy.empty")}</TableCell>
                  </TableRow>
                ) : (
                  rows.map((row) => (
                    <TableRow {...getRowProps({ row })} key={row.id}>
                      {row.cells.map((cell) => (
                        <TableCell key={cell.id}>{cell.value}</TableCell>
                      ))}
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </DataTable>
    </Tile>
  );
}
