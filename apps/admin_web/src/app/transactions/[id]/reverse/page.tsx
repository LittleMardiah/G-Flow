"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { Sidebar } from "@/components/sidebar";
import { Providers } from "@/app/providers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
import { useTransactionDetail, useReverseTransaction } from "@/lib/reversal";
import type { TransactionDetail } from "@/lib/types";

export default function ReverseTransactionPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const router = useRouter();

  const detail = useTransactionDetail(id);
  const reverse = useReverseTransaction();

  const [reason, setReason] = useState("");
  const [notes, setNotes] = useState("");
  const [twoFa, setTwoFa] = useState("");
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const canSubmit =
    !detail.data?.reversed &&
    !reverse.isPending &&
    reason.trim().length > 0 &&
    twoFa.trim().length > 0;

  const openConfirm = () => {
    setErrorMsg(null);
    setConfirmOpen(true);
  };

  const submitReversal = () => {
    setConfirmOpen(false);
    setErrorMsg(null);
    if (typeof window !== "undefined") {
      window.localStorage.setItem("admin_2fa_token", twoFa.trim());
    }
    reverse.mutate(
      { id, reason: reason.trim(), notes: notes.trim() },
      {
        onSuccess: () => router.refresh(),
        onError: () => {
          setErrorMsg("Reversal gagal. Periksa 2FA token atau status transaksi.");
          if (typeof window !== "undefined") {
            window.localStorage.removeItem("admin_2fa_token");
          }
        },
      },
    );
  };

  return (
    <Providers>
      <div className="flex min-h-screen">
        <Sidebar />
        <main className="flex-1 p-6">
          <div className="mb-6 flex items-center gap-3">
            <Link
              href="/ledger"
              className="text-sm text-blue-600 hover:underline"
            >
              ← Back to Ledger
            </Link>
            <h1 className="text-2xl font-bold">Transaction Reversal</h1>
          </div>

          {detail.isLoading && (
            <p className="text-sm text-gray-400">Loading transaction…</p>
          )}
          {detail.isError && (
            <p className="text-sm text-red-500">
              Transaksi tidak ditemukan atau gagal dimuat.
            </p>
          )}

          {detail.data && (
            <div className="space-y-6">
              <TransactionDetailsCard detail={detail.data} />
              <ReversalForm
                reason={reason}
                notes={notes}
                twoFa={twoFa}
                setReason={setReason}
                setNotes={setNotes}
                setTwoFa={setTwoFa}
              />
              {errorMsg && (
                <p className="text-sm text-red-500">{errorMsg}</p>
              )}
              {detail.data.reversed && (
                <p className="text-sm text-amber-600">
                  Transaksi ini sudah pernah di-reverse.
                </p>
              )}
              {detail.data.potential_shortfall > 0 && (
                <p className="mt-4 rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800">
                  <strong>Shortfall:</strong> saldo merchant/driver/platform
                  tidak mencukupi bagiannya. Kekurangan sebesar{" "}
                  {formatAmount(detail.data.potential_shortfall)} akan dicatat
                  sebagai <code>SYSTEM_RECEIVABLE_OVERDRAFT</code>.
                </p>
              )}
              <div className="flex gap-2">
                <Button
                  variant="danger"
                  disabled={!canSubmit}
                  onClick={openConfirm}
                >
                  Confirm Reversal
                </Button>
                {reverse.isSuccess && (
                  <Badge variant="green">Reversal berhasil</Badge>
                )}
              </div>

              {confirmOpen && (
                <ConfirmDialog
                  detail={detail.data}
                  onCancel={() => setConfirmOpen(false)}
                  onConfirm={submitReversal}
                />
              )}
            </div>
          )}
        </main>
      </div>
    </Providers>
  );
}

function TransactionDetailsCard({ detail }: { detail: TransactionDetail }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Transaction Details</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 gap-3 text-sm md:grid-cols-4">
          <div>
            <p className="text-gray-500">Transaction ID</p>
            <p className="break-all font-mono text-xs">{detail.transaction_id}</p>
          </div>
          <div>
            <p className="text-gray-500">Total Amount</p>
            <p className="font-semibold">{formatAmount(detail.total_amount)}</p>
          </div>
          <div>
            <p className="text-gray-500">Refund Wallet</p>
            <p className="break-all font-mono text-xs">{detail.refund_wallet}</p>
          </div>
          <div>
            <p className="text-gray-500">Status</p>
            <Badge variant={detail.reversed ? "gray" : "green"}>
              {detail.reversed ? "REVERSED" : "ACTIVE"}
            </Badge>
          </div>
        </div>

        <div>
          <p className="mb-2 text-sm font-medium text-gray-700">
            Proportional Breakdown
          </p>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            <BreakdownBox
              label="Merchant Share"
              value={detail.merchant_share}
              balance={shareBalance(detail, "MERCHANT")}
            />
            <BreakdownBox
              label="Driver Share"
              value={detail.driver_share}
              balance={shareBalance(detail, "DRIVER")}
            />
            <BreakdownBox
              label="Platform Share"
              value={detail.platform_share}
              balance={shareBalance(detail, "SYSTEM_PLATFORM")}
            />
          </div>
        </div>

        <div>
          <p className="mb-2 text-sm font-medium text-gray-700">Ledger Entries</p>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Wallet</TableHead>
                <TableHead>Entry Type</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Reference</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {detail.entries.map((e, i) => (
                <TableRow key={i}>
                  <TableCell className="font-mono text-xs break-all">
                    {e.wallet_id.slice(0, 12)} ({e.wallet_type || "-"})
                  </TableCell>
                  <TableCell>
                    <Badge variant={entryVariant(e.entry_type)}>
                      {e.entry_type}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-medium">
                    {formatAmount(e.amount)}
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {e.reference}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}

function BreakdownBox({
  label,
  value,
  balance,
}: {
  label: string;
  value: number;
  balance: number;
}) {
  const shortage = balance < value;
  return (
    <div className="rounded-lg border border-gray-200 p-3">
      <p className="text-xs text-gray-500">{label}</p>
      <p className="text-lg font-semibold">{formatAmount(value)}</p>
      <p className={`text-xs ${shortage ? "text-amber-600" : "text-gray-400"}`}>
        Balance: {formatAmount(balance)}
        {shortage ? " (shortfall)" : ""}
      </p>
    </div>
  );
}

function shareBalance(detail: TransactionDetail, walletType: string): number {
  const entry = detail.entries.find(
    (e) =>
      (e.wallet_type ?? "") === walletType &&
      e.entry_type.toUpperCase() === "CREDIT",
  );
  return entry?.balance ?? 0;
}

function ReversalForm({
  reason,
  notes,
  twoFa,
  setReason,
  setNotes,
  setTwoFa,
}: {
  reason: string;
  notes: string;
  twoFa: string;
  setReason: (v: string) => void;
  setNotes: (v: string) => void;
  setTwoFa: (v: string) => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Reversal Detail</CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div>
          <label className="mb-1 block text-sm text-gray-600">Reason *</label>
          <Input
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="e.g. duplicate charge, fraud"
          />
        </div>
        <div>
          <label className="mb-1 block text-sm text-gray-600">Notes</label>
          <Input
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Optional notes"
          />
        </div>
        <div>
          <label className="mb-1 block text-sm text-gray-600">
            2FA Token *
          </label>
          <Input
            type="password"
            value={twoFa}
            onChange={(e) => setTwoFa(e.target.value)}
            placeholder="X-Admin-2FA-Token"
          />
        </div>
      </CardContent>
    </Card>
  );
}

function ConfirmDialog({
  detail,
  onCancel,
  onConfirm,
}: {
  detail: TransactionDetail;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl">
        <h2 className="text-lg font-semibold">Konfirmasi Reversal</h2>
        <p className="mt-2 text-sm text-gray-600">
          Anda akan membalikkan transaksi{" "}
          <span className="font-mono">{detail.transaction_id.slice(0, 12)}…</span>{" "}
          sebesar <strong>{formatAmount(detail.total_amount)}</strong>. Dana akan
          di-clawback proporsional dari merchant, driver, dan platform.
        </p>
        {detail.potential_shortfall > 0 && (
          <div className="mt-3 rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800">
            <strong>Shortfall:</strong> saldo salah satu pihak tidak mencukupi
            bagiannya. Kekurangan {formatAmount(detail.potential_shortfall)}{" "}
            akan dicatat sebagai <code>SYSTEM_RECEIVABLE_OVERDRAFT</code>.
          </div>
        )}
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="outline" onClick={onCancel}>
            Batal
          </Button>
          <Button variant="danger" onClick={onConfirm}>
            Ya, Reverse
          </Button>
        </div>
      </div>
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
