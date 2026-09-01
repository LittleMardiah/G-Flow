"use client";

import { DashboardKPIs } from "./kpi-cards";
import { OrderStatusChart } from "./order-status-chart";
import { ErrorRateChart } from "./error-rate-chart";
import { TransactionList } from "./transaction-list";
import { useDashboardKPIs, useLatestTransactions } from "@/lib/dashboard";

export function DashboardClient() {
  const kpis = useDashboardKPIs();
  const transactions = useLatestTransactions(10);

  const breakdown = kpis.data?.order_status_breakdown ?? [];
  const errorTrend = (
    kpis.data?.error_rate
      ? [
          { timestamp: "now", rate: kpis.data.error_rate },
        ]
      : []
  ) as Array<{ timestamp: string; rate: number }>;

  if (kpis.isLoading) {
    return <p className="text-sm text-gray-400">Loading KPIs…</p>;
  }

  if (kpis.isError) {
    return (
      <p className="text-sm text-red-500">
        Failed to load dashboard KPIs. Is the backend running at the API base URL?
      </p>
    );
  }

  return (
    <div className="space-y-6">
      <DashboardKPIs kpis={kpis.data!} />
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <OrderStatusChart data={breakdown} />
        <ErrorRateChart data={errorTrend} />
      </div>
      <TransactionList transactions={transactions.data?.transactions ?? []} />
    </div>
  );
}
