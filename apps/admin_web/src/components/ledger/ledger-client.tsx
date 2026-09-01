"use client";

import { useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useLedger, useBalanceVerification, useLedgerExport } from "@/lib/ledger";
import type { LedgerFilters } from "@/lib/types";

const PAGE_SIZE = 100;

export function LedgerClient() {
  const [filters, setFilters] = useState<LedgerFilters>({
    offset: 0,
    limit: PAGE_SIZE,
    date_from: "",
    date_to: "",
    wallet_type: "ALL",
    entry_type: "ALL",
    search: "",
  });
  const [verifyWallet, setVerifyWallet] = useState("");
  const [verifyInput, setVerifyInput] = useState("");

  const ledger = useLedger(filters);
  const verification = useBalanceVerification(verifyWallet);
  const ledgerExport = useLedgerExport();

  const update = (patch: Partial<LedgerFilters>) => {
    setFilters((prev) => ({ ...prev, ...patch, offset: 0 }));
  };

  const applySearch = () => {
    setFilters((prev) => ({ ...prev, offset: 0 }));
  };

  const runVerification = () => {
    setVerifyWallet(verifyInput.trim());
  };

  const handleExport = async (format: "csv" | "json") => {
    try {
      const blob = await ledgerExport.mutateAsync({
        ...filters,
        date_from: filters.date_from || undefined,
        date_to: filters.date_to || undefined,
        search: filters.search || undefined,
        wallet_type:
          filters.wallet_type === "ALL" ? undefined : filters.wallet_type,
        entry_type:
          filters.entry_type === "ALL" ? undefined : filters.entry_type,
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `ledger-export.${format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      // export errors surface via mutation state
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Filters</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-5">
            <Input
              type="date"
              value={filters.date_from}
              onChange={(e) => update({ date_from: e.target.value })}
              placeholder="Date from"
            />
            <Input
              type="date"
              value={filters.date_to}
              onChange={(e) => update({ date_to: e.target.value })}
              placeholder="Date to"
            />
            <Select
              value={filters.wallet_type}
              onChange={(e) => update({ wallet_type: e.target.value })}
            >
              <option value="ALL">All Wallet Types</option>
              <option value="CUSTOMER_ESCROW">CUSTOMER_ESCROW</option>
              <option value="RECIPIENT_PAYOUT">RECIPIENT_PAYOUT</option>
              <option value="MERCHANT_PAYOUT">MERCHANT_PAYOUT</option>
              <option value="DRIVER_EARNING">DRIVER_EARNING</option>
              <option value="PLATFORM_SHARE">PLATFORM_SHARE</option>
            </Select>
            <Select
              value={filters.entry_type}
              onChange={(e) => update({ entry_type: e.target.value })}
            >
              <option value="ALL">All Entry Types</option>
              <option value="DEBIT">DEBIT</option>
              <option value="CREDIT">CREDIT</option>
              <option value="HOLD">HOLD</option>
              <option value="RELEASE">RELEASE</option>
              <option value="REVERSAL">REVERSAL</option>
            </Select>
            <div className="flex gap-2">
              <Input
                value={filters.search}
                onChange={(e) =>
                  setFilters((prev) => ({ ...prev, search: e.target.value }))
                }
                onKeyDown={(e) => e.key === "Enter" && applySearch()}
                placeholder="Search reference"
              />
              <Button variant="outline" onClick={applySearch}>
                Search
              </Button>
            </div>
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button
              variant="outline"
              onClick={() => handleExport("csv")}
              disabled={ledgerExport.isPending}
            >
              Export CSV
            </Button>
            <Button
              variant="outline"
              onClick={() => handleExport("json")}
              disabled={ledgerExport.isPending}
            >
              Export JSON
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Balance Verification</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex gap-2">
            <Input
              value={verifyInput}
              onChange={(e) => setVerifyInput(e.target.value)}
              placeholder="Wallet ID"
              className="max-w-sm"
            />
            <Button onClick={runVerification}>Verify</Button>
          </div>
          {verification.data && (
            <div className="mt-4 grid grid-cols-2 gap-3 text-sm md:grid-cols-4">
              <div>
                <p className="text-gray-500">Total Debit</p>
                <p className="font-semibold">{formatAmount(verification.data.total_debit)}</p>
              </div>
              <div>
                <p className="text-gray-500">Total Credit</p>
                <p className="font-semibold">{formatAmount(verification.data.total_credit)}</p>
              </div>
              <div>
                <p className="text-gray-500">Discrepancy</p>
                <p className="font-semibold">{formatAmount(verification.data.discrepancy)}</p>
              </div>
              <div>
                <p className="text-gray-500">Status</p>
                <Badge
                  variant={
                    verification.data.status === "BALANCED" ? "green" : "red"
                  }
                >
                  {verification.data.status}
                </Badge>
              </div>
            </div>
          )}
          {verification.isError && (
            <p className="mt-3 text-sm text-red-500">
              Wallet not found or verification failed.
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Ledger Entries</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Wallet ID</TableHead>
                <TableHead>Entry Type</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Reference</TableHead>
                <TableHead>Created At</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(ledger.data?.ledger ?? []).map((entry) => (
                <TableRow key={entry.id}>
                  <TableCell className="font-mono text-xs">
                    {entry.id.slice(0, 8)}
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {entry.wallet_id.slice(0, 8)}
                  </TableCell>
                  <TableCell>
                    <Badge variant={entryVariant(entry.entry_type)}>
                      {entry.entry_type}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-medium">
                    {formatAmount(entry.amount)}
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {entry.reference}
                  </TableCell>
                  <TableCell className="text-gray-500">
                    {new Date(entry.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    <Link
                      href={`/transactions/${encodeURIComponent(entry.reference)}/reverse`}
                      className="text-sm text-blue-600 hover:underline"
                    >
                      Reverse
                    </Link>
                  </TableCell>
                </TableRow>
              ))}
              {(ledger.data?.ledger ?? []).length === 0 && (
                <TableRow>
                  <TableCell colSpan={7} className="py-6 text-center text-gray-400">
                    No ledger entries
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          <div className="mt-4 flex items-center justify-between">
            <button
              className="text-sm text-blue-600 disabled:text-gray-300"
              disabled={(filters.offset ?? 0) === 0}
              onClick={() =>
                setFilters((prev) => ({
                  ...prev,
                  offset: Math.max(0, (prev.offset ?? 0) - PAGE_SIZE),
                }))
              }
            >
              Previous
            </button>
            <span className="text-sm text-gray-500">
              {ledger.data?.total_count ?? 0} total
            </span>
            <button
              className="text-sm text-blue-600 disabled:text-gray-300"
              disabled={
                (ledger.data?.ledger ?? []).length <
                (filters.limit ?? PAGE_SIZE)
              }
              onClick={() =>
                setFilters((prev) => ({
                  ...prev,
                  offset: (prev.offset ?? 0) + PAGE_SIZE,
                }))
              }
            >
              Next
            </button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function formatAmount(value: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(value ?? 0);
}

function entryVariant(entryType: string): "green" | "red" | "blue" | "gray" {
  const t = entryType.toLowerCase();
  if (t.includes("credit")) return "green";
  if (t.includes("debit")) return "red";
  if (t.includes("hold")) return "blue";
  return "gray";
}
