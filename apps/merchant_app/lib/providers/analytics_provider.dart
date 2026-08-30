import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/merchant_order.dart';
import 'order_provider.dart';

const List<String> kAnalyticsPeriods = ['today', 'week', 'month'];

String analyticsPeriodLabel(String period) {
  switch (period) {
    case 'today':
      return 'Today';
    case 'week':
      return 'This Week';
    case 'month':
      return 'This Month';
    default:
      return period;
  }
}

class TopItem {
  const TopItem({required this.name, required this.quantity});

  final String name;
  final int quantity;
}

class DayRevenue {
  const DayRevenue({required this.date, required this.amount});

  final DateTime date;
  final int amount;
}

class AnalyticsSummary {
  const AnalyticsSummary({
    required this.period,
    this.totalOrders = 0,
    this.totalRevenue = 0,
    this.avgOrderValue = 0,
    this.topItems = const [],
    this.dailyRevenue = const [],
  });

  final String period;
  final int totalOrders;
  final int totalRevenue;
  final int avgOrderValue;
  final List<TopItem> topItems;
  final List<DayRevenue> dailyRevenue;

  factory AnalyticsSummary.compute(List<MerchantOrder> allOrders, String period) {
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);

    final DateTime start;
    switch (period) {
      case 'month':
        start = DateTime(now.year, now.month, 1);
        break;
      case 'week':
        start = today.subtract(const Duration(days: 6));
        break;
      default:
        start = today;
    }

    final orders = allOrders.where((o) {
      final created = o.createdAt;
      if (created == null) return false;
      final d = DateTime(created.year, created.month, created.day);
      return !d.isBefore(start) && !d.isAfter(today);
    }).where((o) => o.displayStatus != 'CANCELLED').toList();

    var revenue = 0;
    final qtyByName = <String, int>{};
    final revenueByDay = <DateTime, int>{};
    for (final o in orders) {
      revenue += o.totalAmount;
      for (final it in o.items) {
        qtyByName[it.itemName] = (qtyByName[it.itemName] ?? 0) + it.quantity;
      }
      if (o.createdAt != null) {
        final day = DateTime(o.createdAt!.year, o.createdAt!.month, o.createdAt!.day);
        revenueByDay[day] = (revenueByDay[day] ?? 0) + o.totalAmount;
      }
    }

    var dayCursor = start;
    final daily = <DayRevenue>[];
    while (!dayCursor.isAfter(today)) {
      daily.add(DayRevenue(date: dayCursor, amount: revenueByDay[dayCursor] ?? 0));
      dayCursor = dayCursor.add(const Duration(days: 1));
    }

    final top = qtyByName.entries.toList()
      ..sort((a, b) => b.value.compareTo(a.value));
    final avg = orders.isEmpty ? 0 : revenue ~/ orders.length;

    return AnalyticsSummary(
      period: period,
      totalOrders: orders.length,
      totalRevenue: revenue,
      avgOrderValue: avg,
      topItems: top.take(5).map((e) => TopItem(name: e.key, quantity: e.value)).toList(),
      dailyRevenue: daily,
    );
  }
}

final analyticsProvider = Provider.family<AnalyticsSummary, String>((ref, period) {
  final orders = ref.watch(orderProvider.select((s) => s.orders));
  return AnalyticsSummary.compute(orders, period);
});

final analyticsTodayProvider = Provider<AnalyticsSummary>((ref) {
  return ref.watch(analyticsProvider('today'));
});