import { ReactNode } from "react";
import { cn } from "../../lib/cn";

export function DataTable({
  className,
  empty,
  headers,
  rows,
  tableClassName,
}: {
  className?: string;
  empty: string;
  headers: string[];
  rows: Array<Array<ReactNode>>;
  tableClassName?: string;
}) {
  return (
    <div className={cn("ui-table-wrap", className)}>
      <table className={cn("ui-table", tableClassName)}>
        <thead>
          <tr>
            {headers.map((header) => (
              <th key={header}>{header}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={headers.length}>{empty}</td>
            </tr>
          ) : (
            rows.map((row, index) => (
              <tr key={index}>
                {row.map((cell, cellIndex) => (
                  <td key={cellIndex}>{cell}</td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
