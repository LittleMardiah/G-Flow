import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

interface KPIProps {
  label: string;
  value: string;
  hint?: string;
}

function KpiCard({ label, value, hint }: KPIProps) {
  return (
    <Card>
      <CardContent className="pt-5">
        <p className="text-xs font-medium text-gray-500">{label}</p>
        <p className="mt-1 text-2xl font-bold">{value}</p>
        {hint && <p className="mt-1 text-xs text-gray-400">{hint}</p>}
      </CardContent>
    </Card>
  );
}

export function DashboardKPIs({
  kpis,
}: {
  kpis: {
    active_orders: number;
    total_transaction_volume: number;
    avg_fare: number;
    revenue_today: number;
    error_rate: number;
  };
}) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <KpiCard label="Active Orders" value={String(kpis.active_orders)} />
      <KpiCard
        label="Total Txn Volume"
        value={formatCurrency(kpis.total_transaction_volume)}
      />
      <KpiCard label="Avg Fare" value={formatCurrency(kpis.avg_fare)} />
      <KpiCard
        label="Revenue Today"
        value={formatCurrency(kpis.revenue_today)}
      />
      <Card>
        <CardContent className="pt-5">
          <p className="text-xs font-medium text-gray-500">Error Rate</p>
          <p className="mt-1 text-2xl font-bold">{kpis.error_rate}%</p>
          <div className="mt-2">
            <Badge variant={kpis.error_rate < 1 ? "green" : "red"}>
              {kpis.error_rate < 1 ? "Healthy" : "High"}
            </Badge>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function formatCurrency(value: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(value ?? 0);
}
