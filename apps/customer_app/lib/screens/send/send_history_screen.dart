import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/send_order.dart';
import '../../providers/send_order_provider.dart';

/// Send Package History (ROADMAP 3.8 & API_CONTRACT 9.4).
class SendHistoryScreen extends ConsumerStatefulWidget {
  const SendHistoryScreen({super.key});

  @override
  ConsumerState<SendHistoryScreen> createState() => _SendHistoryScreenState();
}

class _SendHistoryScreenState extends ConsumerState<SendHistoryScreen> {
  final ScrollController _scroll = ScrollController();

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(sendHistoryProvider.notifier).loadFirst());
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
      ref.read(sendHistoryProvider.notifier).loadMore();
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(sendHistoryProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Riwayat Pengiriman')),
      body: state.isLoading && state.orders.isEmpty
          ? const Center(child: CircularProgressIndicator())
          : state.error != null && state.orders.isEmpty
              ? _ErrorView(
                  message: state.error!,
                  onRetry: () =>
                      ref.read(sendHistoryProvider.notifier).loadFirst(),
                )
              : state.orders.isEmpty
                  ? const Center(child: Text('Belum ada pengiriman.'))
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
                        return _SendTile(order: state.orders[i]);
                      },
                    ),
    );
  }
}

class _SendTile extends StatelessWidget {
  const _SendTile({required this.order});

  final SendOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final color = _statusColor(order.status);
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
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
                Expanded(
                  child: Text('#${_shortId(order.id)}',
                      style: theme.textTheme.titleSmall),
                ),
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(
                    color: color.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Text(order.status.replaceAll('_', ' '),
                      style: TextStyle(color: color, fontSize: 11)),
                ),
              ],
            ),
            const SizedBox(height: 6),
            Text('Package: ${order.packageType}',
                style: theme.textTheme.bodyMedium),
            if (order.totalDistanceKm > 0)
              Text(
                  '${order.totalDistanceKm.toStringAsFixed(1)} km · '
                  '${order.stops.length} stop',
                  style: theme.textTheme.bodySmall),
            const Divider(height: 16),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(_formatRupiah(order.totalAmount),
                    style: theme.textTheme.titleSmall
                        ?.copyWith(fontWeight: FontWeight.bold)),
                TextButton(
                  onPressed: () => Navigator.of(context).pushNamed(
                      '/send-tracking',
                      arguments: SendOrderTrackingArgs(
                        orderId: order.id,
                        totalAmount: order.totalAmount,
                        packageType: order.packageType,
                      )),
                  child: const Text('Lacak'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

Color _statusColor(String status) {
  switch (status) {
    case 'DELIVERED':
    case 'SETTLED':
      return Colors.green;
    case 'CANCELLED':
    case 'RETURN_REQUIRED':
    case 'RETURNED_TO_SENDER':
      return Colors.red;
    default:
      return Colors.orange;
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
              label: const Text('Coba lagi')),
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
