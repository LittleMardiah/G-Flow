import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../config/constants.dart';
import '../models/driver_order.dart';
import '../providers/order_provider.dart';
import '../widgets/kpi_card.dart';
import '../widgets/order_type_badge.dart';

enum _SortMode { distance, type, pickupTime }

/// Available Orders Screen (ROADMAP 3.10 B/F).
///
/// List semua order tersedia dari semua service (ride + food + send),
/// filter per tipe, sortir berdasarkan jarak/tipe/waktu pickup, dan tombol
/// ACCEPT dengan countdown (+ capacity check via activeOrdersProvider).
class AvailableOrdersScreen extends ConsumerStatefulWidget {
  const AvailableOrdersScreen({super.key});

  @override
  ConsumerState<AvailableOrdersScreen> createState() => _AvailableOrdersScreenState();
}

class _AvailableOrdersScreenState extends ConsumerState<AvailableOrdersScreen> {
  OrderType? _filter;
  _SortMode _sort = _SortMode.distance;
  bool _accepting = false;
  String? _message;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(availableOrdersProvider.notifier).fetch();
    });
  }

  List<DriverOrder> _sorted(List<DriverOrder> orders) {
    var list = _filter == null
        ? List<DriverOrder>.from(orders)
        : orders.where((o) => o.type == _filter).toList();
    switch (_sort) {
      case _SortMode.distance:
        list.sort((a, b) => a.distanceKm.compareTo(b.distanceKm));
      case _SortMode.type:
        list.sort((a, b) => a.type.index.compareTo(b.type.index));
      case _SortMode.pickupTime:
        list.sort((a, b) {
          final ta = a.createdAt;
          final tb = b.createdAt;
          if (ta == null && tb == null) return 0;
          if (ta == null) return 1;
          if (tb == null) return -1;
          return ta.compareTo(tb);
        });
    }
    return list;
  }

  Future<void> _accept(DriverOrder order) async {
    setState(() {
      _accepting = true;
      _message = null;
    });
    final ok = await ref.read(activeOrdersProvider.notifier).accept(order);
    if (!mounted) return;
    setState(() => _accepting = false);
    if (ok) {
      ref.read(availableOrdersProvider.notifier).remove(order.id);
      setState(() => _message = 'Order ${order.type.label} #${shortId(order.id)} diterima.');
    } else {
      setState(() => _message = 'Gagal menerima order. Kapasitas penuh atau terjadi error.');
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(availableOrdersProvider);
    final active = ref.watch(activeOrdersProvider);
    final orders = _sorted(state.orders);
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Pesanan Tersedia'),
        actions: [
          if (state.orders.any((o) => o.isMock))
            const DemoBadge(tooltip: 'Mode demo: endpoint GET /drivers/available-orders belum ada di backend.'),
        ],
      ),
      body: Column(
        children: [
          _FilterBar(
            filter: _filter,
            sort: _sort,
            onFilter: (t) => setState(() => _filter = t),
            onSort: (s) => setState(() => _sort = s),
          ),
          if (_message != null)
            Container(
              width: double.infinity,
              color: theme.colorScheme.primaryContainer,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              child: Text(
                _message!,
                style: TextStyle(color: theme.colorScheme.onPrimaryContainer, fontSize: 12),
              ),
            ),
          if (active.isAtCapacity)
            Container(
              width: double.infinity,
              color: Colors.orange.shade50,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              child: Text(
                'Capacity Reached: ${active.activeCount}/$kMaxActiveOrders order aktif. Accept ditolak.',
                style: TextStyle(color: Colors.orange.shade800, fontSize: 12, fontWeight: FontWeight.w600),
              ),
            ),
          if (active.error != null)
            Container(
              width: double.infinity,
              color: Colors.red.shade50,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              child: Text(active.error!, style: TextStyle(color: Colors.red.shade800, fontSize: 12)),
            ),
          Expanded(
            child: state.isLoading
                ? const Center(child: CircularProgressIndicator())
                : state.error != null
                    ? EmptyStateView(
                        icon: Icons.cloud_off,
                        message: 'Gagal memuat pesanan.\n${state.error!}',
                      )
                    : orders.isEmpty
                        ? const EmptyStateView(
                            icon: Icons.inbox_outlined,
                            message: 'Tidak ada pesanan tersedia saat ini.',
                          )
                        : RefreshIndicator(
                            onRefresh: () => ref.read(availableOrdersProvider.notifier).fetch(),
                            child: ListView.builder(
                              padding: const EdgeInsets.all(12),
                              itemCount: orders.length,
                              itemBuilder: (context, i) => _OrderCard(
                                order: orders[i],
                                accepting: _accepting,
                                disabled: active.isAtCapacity,
                                onAccept: () => _accept(orders[i]),
                              ),
                            ),
                          ),
          ),
        ],
      ),
    );
  }
}

class _FilterBar extends StatelessWidget {
  const _FilterBar({
    required this.filter,
    required this.sort,
    required this.onFilter,
    required this.onSort,
  });

  final OrderType? filter;
  final _SortMode sort;
  final ValueChanged<OrderType?> onFilter;
  final ValueChanged<_SortMode> onSort;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
      child: Row(
        children: [
          ChoiceChip(
            label: const Text('Semua'),
            selected: filter == null,
            onSelected: (_) => onFilter(null),
          ),
          const SizedBox(width: 6),
          for (final t in OrderType.values)
            Padding(
              padding: const EdgeInsets.only(left: 0, right: 6),
              child: ChoiceChip(
                label: Text(t.label),
                selected: filter == t,
                onSelected: (_) => onFilter(t),
              ),
            ),
          const Spacer(),
          PopupMenuButton<_SortMode>(
            icon: const Icon(Icons.sort),
            tooltip: 'Urutkan',
            initialValue: sort,
            onSelected: onSort,
            itemBuilder: (_) => const [
              PopupMenuItem(value: _SortMode.distance, child: Text('Jarak terdekat')),
              PopupMenuItem(value: _SortMode.type, child: Text('Tipe order')),
              PopupMenuItem(value: _SortMode.pickupTime, child: Text('Waktu pickup')),
            ],
          ),
        ],
      ),
    );
  }
}

class _OrderCard extends ConsumerStatefulWidget {
  const _OrderCard({
    required this.order,
    required this.accepting,
    required this.disabled,
    required this.onAccept,
  });

  final DriverOrder order;
  final bool accepting;
  final bool disabled;
  final VoidCallback onAccept;

  @override
  ConsumerState<_OrderCard> createState() => _OrderCardState();
}

class _OrderCardState extends ConsumerState<_OrderCard> {
  static const int _countdownSeconds = 30;
  Timer? _timer;
  int _secondsLeft = _countdownSeconds;

  @override
  void initState() {
    super.initState();
    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      setState(() {
        _secondsLeft = (_secondsLeft - 1).clamp(0, _countdownSeconds);
        if (_secondsLeft == 0) _timer?.cancel();
      });
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final order = widget.order;
    final theme = Theme.of(context);
    final expired = _secondsLeft == 0;

    return Card(
      margin: const EdgeInsets.symmetric(vertical: 5),
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                OrderTypeBadge(type: order.type),
                const SizedBox(width: 8),
                Text('#${shortId(order.id)}', style: theme.textTheme.labelMedium),
                const Spacer(),
                _CountdownChip(secondsLeft: _secondsLeft, expired: expired),
              ],
            ),
            const SizedBox(height: 8),
            if (order.type == OrderType.food) ...[
              _row(Icons.storefront, 'Merchant: ${order.merchantName.isNotEmpty ? order.merchantName : order.merchantAddress}'),
              _row(Icons.arrow_downward, 'Kirim ke: ${order.deliveryAddress}'),
            ] else ...[
              _row(Icons.arrow_upward, 'Pickup: ${order.pickupAddress}'),
              _row(
                Icons.arrow_downward,
                order.type == OrderType.send
                    ? 'Kirim (${order.stops.length} stop): ${order.deliveryAddress}'
                    : 'Tujuan: ${order.deliveryAddress}',
              ),
            ],
            const SizedBox(height: 4),
            if (order.type == OrderType.send && order.hasStops) ...[
              for (final s in order.stops)
                Padding(
                  padding: const EdgeInsets.only(left: 8, bottom: 2),
                  child: Text(
                    '  ${s.stopNumber}. ${s.address} (${formatRupiah(s.allocatedFare)})',
                    style: theme.textTheme.bodySmall?.copyWith(color: Colors.grey),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              const SizedBox(height: 4),
            ],
            Row(
              children: [
                const Icon(Icons.route_outlined, size: 14, color: Colors.grey),
                const SizedBox(width: 4),
                Text('${order.distanceKm.toStringAsFixed(1)} km', style: theme.textTheme.bodySmall),
                const Spacer(),
                const Icon(Icons.savings_outlined, size: 14, color: Colors.green),
                const SizedBox(width: 4),
                Text(
                  formatRupiah(order.driverEarning > 0 ? order.driverEarning : _estimatedEarning(order)),
                  style: theme.textTheme.bodySmall?.copyWith(color: Colors.green.shade800, fontWeight: FontWeight.bold),
                ),
              ],
            ),
            const SizedBox(height: 10),
            SizedBox(
              width: double.infinity,
              height: 40,
              child: FilledButton.icon(
                style: FilledButton.styleFrom(
                  backgroundColor: theme.colorScheme.primary,
                  disabledBackgroundColor: theme.colorScheme.surfaceContainerHighest,
                ),
                onPressed: widget.accepting || expired || widget.disabled ? null : widget.onAccept,
                icon: widget.accepting
                    ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.check_circle_outline, size: 18),
                label: Text(
                  widget.accepting
                      ? 'Menerima…'
                      : expired
                          ? 'Kedaluwarsa'
                          : widget.disabled
                              ? 'Kapasitas penuh'
                              : 'ACCEPT',
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  static int _estimatedEarning(DriverOrder o) {
    if (o.driverEarning > 0) return o.driverEarning;
    return (o.estimatedFare * 0.8).round();
  }
}

class _CountdownChip extends StatelessWidget {
  const _CountdownChip({required this.secondsLeft, required this.expired});

  final int secondsLeft;
  final bool expired;

  @override
  Widget build(BuildContext context) {
    final color = expired ? Colors.grey : Colors.deepOrange;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(expired ? Icons.timer_off : Icons.timer_outlined, size: 13, color: color),
          const SizedBox(width: 4),
          Text(
            expired ? '00:00' : _fmt(secondsLeft),
            style: TextStyle(color: color, fontSize: 12, fontWeight: FontWeight.bold),
          ),
        ],
      ),
    );
  }

  static String _fmt(int s) {
    final mm = (s ~/ 60).toString().padLeft(2, '0');
    final ss = (s % 60).toString().padLeft(2, '0');
    return '$mm:$ss';
  }
}

Widget _row(IconData icon, String text) {
  return Padding(
    padding: const EdgeInsets.only(bottom: 4),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 14, color: Colors.grey),
        const SizedBox(width: 6),
        Expanded(child: Text(text, style: const TextStyle(fontSize: 13), overflow: TextOverflow.ellipsis)),
      ],
    ),
  );
}