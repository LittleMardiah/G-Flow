import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../config/constants.dart';
import '../models/driver_earning.dart';
import '../providers/auth_provider.dart';
import '../providers/earnings_provider.dart';
import '../widgets/kpi_card.dart';

/// Earnings Screen (ROADMAP 3.10 & HALAMAN 7.1).
///
/// Chart harian/mingguan (fl_chart bar), total order ride/food/send,
/// dan tombol withdrawal (placeholder — endpoint pull belum dibangun).
class EarningsScreen extends ConsumerStatefulWidget {
  const EarningsScreen({super.key});

  @override
  ConsumerState<EarningsScreen> createState() => _EarningsScreenState();
}

class _EarningsScreenState extends ConsumerState<EarningsScreen> {
  bool _weekly = false;
  bool _withdrawing = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      final driverId = await ref.read(currentDriverIdProvider.future);
      if (driverId != null && driverId.isNotEmpty) {
        ref.read(earningsProvider.notifier).fetch(driverId);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(earningsProvider);
    final earning = state.earning;
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Earnings'),
        actions: [
          if (earning?.isMock ?? false)
            const DemoBadge(tooltip: 'Mode demo: endpoint GET /drivers/earnings belum ada di backend.'),
        ],
      ),
      body: state.isLoading
          ? const Center(child: CircularProgressIndicator())
          : state.error != null
              ? EmptyStateView(icon: Icons.cloud_off, message: 'Gagal memuat earnings.\n${state.error!}')
              : earning == null
                  ? const EmptyStateView(icon: Icons.payments_outlined, message: 'Belum ada data earnings.')
                  : RefreshIndicator(
                      onRefresh: () async {
                        final driverId = await ref.read(currentDriverIdProvider.future);
                        if (driverId != null && driverId.isNotEmpty) {
                          ref.read(earningsProvider.notifier).fetch(driverId);
                        }
                      },
                      child: ListView(
                        padding: const EdgeInsets.all(16),
                        children: [
                          Row(
                            children: [
                              Expanded(
                                child: KpiCard(
                                  icon: Icons.payments_outlined,
                                  label: _weekly ? 'Pekan ini' : 'Hari ini',
                                  value: formatRupiah(_weekly ? earning.weekTotal : earning.todayTotal),
                                  iconColor: Colors.green,
                                  valueColor: Colors.green.shade800,
                                ),
                              ),
                              const SizedBox(width: 12),
                              Expanded(
                                child: KpiCard(
                                  icon: Icons.receipt_long_outlined,
                                  label: 'Total order',
                                  value: '${earning.todayOrderCount}',
                                  iconColor: Colors.indigo,
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 12),
                          Row(
                            children: [
                              _TypeCount(icon: Icons.directions_car, color: Colors.blue, label: 'Ride', count: earning.rideCount),
                              _TypeCount(icon: Icons.restaurant, color: Colors.orange, label: 'Food', count: earning.foodCount),
                              _TypeCount(icon: Icons.inventory_2, color: Colors.green, label: 'Send', count: earning.sendCount),
                            ],
                          ),
                          const SizedBox(height: 16),
                          SegmentedButton<bool>(
                            segments: const [
                              ButtonSegment(value: false, label: Text('Harian')),
                              ButtonSegment(value: true, label: Text('Mingguan')),
                            ],
                            selected: {_weekly},
                            onSelectionChanged: (s) => setState(() => _weekly = s.first),
                          ),
                          const SizedBox(height: 12),
                          Container(
                            height: 220,
                            padding: const EdgeInsets.all(12),
                            decoration: BoxDecoration(
                              color: theme.colorScheme.surfaceContainerLow,
                              borderRadius: BorderRadius.circular(12),
                            ),
                            child: _EarningsChart(points: _weekly ? earning.weekly : earning.daily),
                          ),
                          const SizedBox(height: 16),
                          FilledButton.icon(
                            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(48)),
                            onPressed: _withdrawing ? null : () => _withdraw(),
                            icon: _withdrawing
                                ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
                                : const Icon(Icons.currency_exchange),
                            label: Text(_withdrawing ? 'Memproses…' : 'Tarik Pendapatan (Withdrawal)'),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            'Withdrawal transfer ke rekening Anda; endpoint pull dana dijadwalkan di phase berikutnya (admin).',
                            textAlign: TextAlign.center,
                            style: theme.textTheme.bodySmall?.copyWith(color: Colors.grey),
                          ),
                        ],
                      ),
                    ),
    );
  }

  Future<void> _withdraw() async {
    setState(() => _withdrawing = true);
    await Future<void>.delayed(const Duration(seconds: 1));
    if (!mounted) return;
    setState(() => _withdrawing = false);
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('Fitur withdrawal belum tersedia di phase ini (lihat ROADMAP 04).')),
    );
  }
}

class _TypeCount extends StatelessWidget {
  const _TypeCount({
    required this.icon,
    required this.color,
    required this.label,
    required this.count,
  });

  final IconData icon;
  final Color color;
  final String label;
  final int count;

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: Container(
        margin: const EdgeInsets.symmetric(horizontal: 4),
        padding: const EdgeInsets.symmetric(vertical: 10),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Column(
          children: [
            Icon(icon, color: color, size: 20),
            const SizedBox(height: 4),
            Text('$count', style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 15)),
            Text(label, style: const TextStyle(fontSize: 11, color: Colors.grey)),
          ],
        ),
      ),
    );
  }
}

class _EarningsChart extends StatelessWidget {
  const _EarningsChart({required this.points});

  final List<EarningPoint> points;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final maxY = points.fold(0, (max, p) => p.amount > max ? p.amount : max);
    final data = points.isEmpty
        ? const <BarChartGroupData>[]
        : [
            for (var i = 0; i < points.length; i++)
              BarChartGroupData(
                x: i,
                barRods: [
                  BarChartRodData(
                    toY: points[i].amount.toDouble(),
                    color: Colors.green.shade500,
                    width: 10,
                    borderRadius: BorderRadius.circular(3),
                    backDrawRodData: BackgroundBarChartRodData(
                      show: true,
                      toY: maxY == 0 ? 1 : maxY.toDouble(),
                      color: theme.colorScheme.surfaceContainerHighest,
                    ),
                  ),
                ],
              ),
          ];

    return BarChart(
      BarChartData(
        maxY: maxY == 0 ? 1 : (maxY * 1.2).toDouble(),
        alignment: BarChartAlignment.spaceAround,
        barTouchData: BarTouchData(
          touchTooltipData: BarTouchTooltipData(
            getTooltipItem: (group, groupIndex, rod, rodIndex) {
              final p = points[group.x];
              return BarTooltipItem(
                '${p.label}\n${formatRupiah(p.amount)}',
                const TextStyle(color: Colors.white, fontSize: 11),
              );
            },
          ),
        ),
        titlesData: FlTitlesData(
          leftTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
          topTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
          rightTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
          bottomTitles: AxisTitles(
            sideTitles: SideTitles(
              showTitles: true,
              reservedSize: 26,
              getTitlesWidget: (value, meta) {
                final i = value.toInt();
                if (i < 0 || i >= points.length) return const SizedBox.shrink();
                return Padding(
                  padding: const EdgeInsets.only(top: 4),
                  child: Text(
                    points[i].label,
                    style: const TextStyle(fontSize: 10, color: Colors.grey),
                  ),
                );
              },
            ),
          ),
        ),
        borderData: FlBorderData(show: false),
        gridData: const FlGridData(show: false),
        barGroups: data,
      ),
    );
  }
}