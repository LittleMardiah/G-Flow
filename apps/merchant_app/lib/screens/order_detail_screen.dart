import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../config/constants.dart';
import '../models/merchant_order.dart';
import '../providers/order_provider.dart';
import '../widgets/status_badge.dart';

class OrderDetailScreen extends ConsumerStatefulWidget {
  const OrderDetailScreen({super.key, required this.orderId});

  final String orderId;

  @override
  ConsumerState<OrderDetailScreen> createState() => _OrderDetailScreenState();
}

class _OrderDetailScreenState extends ConsumerState<OrderDetailScreen> {
  bool _submitting = false;

  Future<void> _apply(MerchantOrderAction action) async {
    setState(() => _submitting = true);
    final ok = await ref.read(orderProvider.notifier).setStatus(widget.orderId, action.targetStatus);
    if (!mounted) return;
    setState(() => _submitting = false);
    ref.invalidate(orderDetailProvider(widget.orderId));
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(ok
            ? 'Status diperbarui: ${action.label}'
            : 'Gagal memperbarui status. Cek koneksi/permission.'),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final detailAsync = ref.watch(orderDetailProvider(widget.orderId));
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(title: const Text('Detail Order')),
      body: detailAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('Gagal memuat detail: $e', textAlign: TextAlign.center),
                const SizedBox(height: 16),
                FilledButton.icon(
                  onPressed: () => ref.invalidate(orderDetailProvider(widget.orderId)),
                  icon: const Icon(Icons.refresh),
                  label: const Text('Coba Lagi'),
                ),
              ],
            ),
          ),
        ),
        data: (order) => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Card(
              elevation: 0,
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Text(order.idShort, style: theme.textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800)),
                        const Spacer(),
                        StatusBadge(status: order.displayStatus),
                      ],
                    ),
                    const SizedBox(height: 8),
                    if (order.createdAt != null)
                      Text('Dibuat: ${_fmtDateTime(order.createdAt!)}', style: theme.textTheme.bodySmall),
                    if (order.customerId != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 4),
                        child: Text('Customer: ${order.customerId!}', style: theme.textTheme.bodySmall),
                      ),
                    const SizedBox(height: 8),
                    _InfoRow(label: 'Metode bayar', value: order.paymentMethod),
                    if (order.deliveryAddress.isNotEmpty)
                      _InfoRow(label: 'Alamat', value: order.deliveryAddress),
                    if (order.specialInstructions != null && order.specialInstructions!.isNotEmpty)
                      _InfoRow(label: 'Catatan pelanggan', value: order.specialInstructions!),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 12),
            Text('Items', style: theme.textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w800)),
            const SizedBox(height: 8),
            Card(
              elevation: 0,
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  children: [
                    for (final item in order.items) _ItemRow(item: item),
                    const Divider(),
                    _AmountRow(label: 'Subtotal', value: formatRupiah(order.itemSubtotal)),
                    _AmountRow(label: 'Delivery fee', value: formatRupiah(order.deliveryFee)),
                    if (order.discountAmount > 0)
                      _AmountRow(label: 'Diskon', value: '-${formatRupiah(order.discountAmount)}'),
                    const Divider(),
                    _AmountRow(label: 'Total', value: formatRupiah(order.totalAmount), bold: true),
                  ],
                ),
              ),
            ),
            if (order.merchantNotes != null && order.merchantNotes!.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text('Catatan merchant: ${order.merchantNotes!}'),
            ],
            if (order.nextActions.isNotEmpty) ...[
              const SizedBox(height: 20),
              FilledButton.icon(
                onPressed: _submitting ? null : () => _apply(order.nextActions.first),
                icon: const Icon(Icons.check_circle_outline),
                label: Text(order.nextActions.first.label),
              ),
              if (order.nextActions.length > 1) ...[
                const SizedBox(height: 8),
                OutlinedButton.icon(
                  onPressed: _submitting ? null : () => _apply(order.nextActions.last),
                  style: OutlinedButton.styleFrom(foregroundColor: theme.colorScheme.error),
                  icon: const Icon(Icons.cancel_outlined),
                  label: Text(order.nextActions.last.label),
                ),
              ],
            ],
            const SizedBox(height: 24),
          ],
        ),
      ),
    );
  }

  String _fmtDateTime(DateTime dt) {
    final local = dt.toLocal();
    final m = local.month.toString().padLeft(2, '0');
    final d = local.day.toString().padLeft(2, '0');
    final h = local.hour.toString().padLeft(2, '0');
    final min = local.minute.toString().padLeft(2, '0');
    return '$d/$m/${local.year} $h:$min';
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 120, child: Text(label, style: theme.textTheme.bodySmall?.copyWith(color: theme.colorScheme.onSurfaceVariant))),
          Expanded(child: Text(value, style: theme.textTheme.bodySmall)),
        ],
      ),
    );
  }
}

class _ItemRow extends StatelessWidget {
  const _ItemRow({required this.item});

  final MerchantOrderItem item;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('${item.quantity}x ${item.itemName}', style: theme.textTheme.bodyMedium),
                if (item.selectedOptions.isNotEmpty)
                  Text(item.selectedOptions.join(', '),
                      style: theme.textTheme.bodySmall?.copyWith(color: theme.colorScheme.onSurfaceVariant)),
              ],
            ),
          ),
          Text(formatRupiah(item.subtotal), style: theme.textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }
}

class _AmountRow extends StatelessWidget {
  const _AmountRow({required this.label, required this.value, this.bold = false});

  final String label;
  final String value;
  final bool bold;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final weight = bold ? FontWeight.w800 : FontWeight.w500;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        children: [
          Text(label, style: theme.textTheme.bodyMedium?.copyWith(fontWeight: weight)),
          const Spacer(),
          Text(value, style: theme.textTheme.bodyMedium?.copyWith(fontWeight: weight)),
        ],
      ),
    );
  }
}