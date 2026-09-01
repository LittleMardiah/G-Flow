"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import type { Transaction } from "@/lib/types";

function entryVariant(entryType: string): "green" | "red" | "blue" | "gray" {
  const t = entryType.toLowerCase();
  if (t.includes("credit")) return "green";
  if (t.includes("debit")) return "red";
  if (t.includes("hold")) return "blue";
  return "gray";
}

export function TransactionList({ transactions }: { transactions: Transaction[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Latest Transactions</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>Entry Type</TableHead>
              <TableHead>Amount</TableHead>
              <TableHead>Reference</TableHead>
              <TableHead>Created At</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {transactions.map((t) => (
              <TableRow key={t.id}>
                <TableCell className="font-mono text-xs">{t.id.slice(0, 8)}</TableCell>
                <TableCell>
                  <Badge variant={entryVariant(t.entry_type)}>{t.entry_type}</Badge>
                </TableCell>
                <TableCell className="font-medium">
                  {formatAmount(t.amount)}
                </TableCell>
                <TableCell className="font-mono text-xs">{t.reference}</TableCell>
                <TableCell className="text-gray-500">
                  {new Date(t.created_at).toLocaleString()}
                </TableCell>
              </TableRow>
            ))}
            {transactions.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-6 text-center text-gray-400">
                  No transactions
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

function formatAmount(value: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(value ?? 0);
}
