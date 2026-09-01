"use client";

import {
  LineChart,
  Line,
  ResponsiveContainer,
  XAxis,
  YAxis,
  Tooltip,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export function ErrorRateChart({
  data,
}: {
  data: Array<{ timestamp: string; rate: number }>;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Error Rate Trend</CardTitle>
      </CardHeader>
      <CardContent className="h-64">
        {data && data.length > 0 ? (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data}>
              <XAxis dataKey="timestamp" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Line
                type="monotone"
                dataKey="rate"
                stroke="#ef4444"
                strokeWidth={2}
              />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <p className="flex h-full items-center justify-center text-sm text-gray-400">
            No data
          </p>
        )}
      </CardContent>
    </Card>
  );
}
