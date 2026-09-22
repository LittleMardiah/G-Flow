import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../config/router.dart';
import '../../models/ride_order.dart';
import '../../providers/ride_provider.dart';

/// Ride History (TD-072 C3, API_CONTRACT 7.4) dengan pagination via
/// scroll listener (pola sama food_history_screen.dart). Tap kartu →
/// `/ride-detail`.
class RideHistoryScreen extends ConsumerStatefulWidget {
  const RideHistoryScreen({super.key});

  @override
  ConsumerState<RideHistoryScreen> createState() => _RideHistoryScreenState();
}

class _RideHistoryScreenState extends ConsumerState<RideHistoryScreen> {
  final ScrollController _scroll = ScrollController();

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(rideHistoryProvider.notifier).loadFirst());
    _scroll.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scroll.removeListener(_onScroll);
    _scroll.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scroll.position.pixels >= _scroll.position.maxScrollExtent - 200) {
      ref.read(rideHistoryProvider.notifier).loadMore();
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(rideHistoryProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Riwayat Perjalanan')),
      body: state.isLoading && state.orders.isEmpty
          ? const Center(child: CircularProgressIndicator())
          : state.error != null && state.orders.isEmpty
              ? _ErrorView(
                  message: state.error!,
                  onRetry: () => ref.read(rideHistoryProvider.notifier).loadFirst(),
                )
              : state.orders.isEmpty
                  ? const Center(child: Text('Belum ada perjalanan.'))
                  : ListView.builder(
                      controller: _scroll,
                      padding: const EdgeInsets.all(16),
                      itemCount: state.orders.length + (state.hasMore ? 1 : 0),
                      itemBuilder: (context, i) {
                        if (i >= state.orders.length) {
                          return const Padding(
                            padding: EdgeInsets.all(16),
                            child: Center(child: CircularProgressIndicator()),
                          );
                        }
                        return _RideTile(order: state.orders[i]);
                      },
                    ),
    );
  }
}

class _RideTile extends StatelessWidget {
  const _RideTile({required this.order});

  final RideOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = _statusColor(order.status);
    final fare = order.actualFare ?? order.estimatedFare;
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: () => Navigator.of(context).pushNamed(
          AppRoutes.rideDetail,
          arguments: RideDetailArgs(orderId: order.id),
        ),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      '#${_shortId(order.id)}',
                      style: theme.textTheme.titleSmall,
                    ),
                  ),
                  if (order.createdAt != null)
                    Text(
                      _formatDate(order.createdAt!),
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  const SizedBox(width: 8),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(
                      color: color.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Text(
                      order.status.replaceAll('_', ' '),
                      style: TextStyle(color: color, fontSize: 11),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                order.pickupAddress.isNotEmpty ? order.pickupAddress : _coord(order.pickupLat, order.pickupLng),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodySmall,
              ),
              Row(
                children: [
                  const Icon(Icons.arrow_downward, size: 14),
                  const SizedBox(width: 4),
                  Expanded(
                    child: Text(
                      order.dropoffAddress.isNotEmpty
                          ? order.dropoffAddress
                          : _coord(order.dropoffLat, order.dropoffLng),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: theme.textTheme.bodySmall,
                    ),
                  ),
                ],
              ),
              const Divider(height: 16),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    _formatRupiah(fare),
                    style: theme.textTheme.titleSmall?.copyWith(fontWeight: FontWeight.bold),
                  ),
                  const Text('Lihat Detail',
                      style: TextStyle(color: Colors.indigo, fontSize: 12)),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

Color _statusColor(String status) {
  switch (status) {
    case 'COMPLETED':
    case 'SETTLED':
      return Colors.green;
    case 'CANCELLED':
      return Colors.red;
    case 'SEARCHING_DRIVER':
      return Colors.orange;
    default:
      return Colors.indigo;
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.cloud_off, size: 48, color: Colors.grey),
          const SizedBox(height: 12),
          Text(message, textAlign: TextAlign.center),
          const SizedBox(height: 16),
          FilledButton.icon(
            onPressed: onRetry,
            icon: const Icon(Icons.refresh),
            label: const Text('Coba lagi'),
          ),
        ],
      ),
    );
  }
}

String _shortId(String id) =>
    id.length > 8 ? id.substring(0, 8).toUpperCase() : id.toUpperCase();

String _formatRupiah(num value) {
  final digits = value.round().toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buffer.write('.');
    buffer.write(digits[i]);
  }
  return 'Rp $buffer';
}

String _formatDate(DateTime dt) {
  final local = dt.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(local.day)}/${two(local.month)}/${local.year} '
      '${two(local.hour)}:${two(local.minute)}';
}

String _coord(double lat, double lng) =>
    '${lat.toStringAsFixed(5)}, ${lng.toStringAsFixed(5)}';