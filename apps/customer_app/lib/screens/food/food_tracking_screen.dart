import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/food_order.dart';
import '../../providers/food_order_provider.dart';

/// Food Order Tracking (ROADMAP 3.8 E & HALAMAN.txt 4.4).
class FoodTrackingScreen extends ConsumerWidget {
  const FoodTrackingScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final args = _args(context);
    if (args == null) return const _MissingArgsView();

    final state = ref.watch(foodTrackingProvider(args));
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Text('Order #${_shortId(args.orderId)}'),
        actions: [
          if (state.isMock) const _DemoBadge(),
        ],
      ),
      body: state.order == null
          ? (state.error != null
              ? _ErrorView(
                  message: state.error!,
                  onRetry: () =>
                      ref.read(foodTrackingProvider(args).notifier).retry(),
                )
              : const Center(child: CircularProgressIndicator()))
          : _content(context, ref, args, state, theme),
    );
  }

  Widget _content(BuildContext context, WidgetRef ref,
      FoodOrderTrackingArgs args, FoodTrackingState state, ThemeData theme) {
    final order = state.order!;
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        if (state.error != null)
          Container(
            width: double.infinity,
            color: Colors.red.shade50,
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            child: Text(state.error!,
                style: TextStyle(color: Colors.red.shade800, fontSize: 12)),
          ),
        _StatusCard(order: order),
        const SizedBox(height: 14),
        _MerchantCard(order: order),
        const SizedBox(height: 14),
        if (order.status == 'PICKED_UP' || order.status == 'IN_TRANSIT')
          const _DriverCard(),
        const SizedBox(height: 14),
        _OrderSummaryCard(order: order),
        const SizedBox(height: 20),
        if (order.isTerminal)
          _TerminalBox(order: order,
              onDone: () => Navigator.of(context).pop())
        else
          _CancelButton(
            onPressed: () => _confirmCancel(context, ref, args),
          ),
      ],
    );
  }

  FoodOrderTrackingArgs? _args(BuildContext context) {
    final raw = ModalRoute.of(context)?.settings.arguments;
    return raw is FoodOrderTrackingArgs ? raw : null;
  }

  Future<void> _confirmCancel(BuildContext context, WidgetRef ref, args) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (d) => AlertDialog(
        title: const Text('Batalkan pesanan?'),
        content: const Text(
            'Pembatalan sebelum konfirmasi restoran tidak dikenakan biaya. '
            'Dana escrow (jika WALLET) akan dikembalikan.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.of(d).pop(false),
              child: const Text('Kembali')),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Colors.red.shade700),
            onPressed: () => Navigator.of(d).pop(true),
            child: const Text('Ya, batalkan'),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;
    final ok = await ref.read(foodTrackingProvider(args).notifier).cancelOrder();
    if (ok && context.mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Pesanan dibatalkan.')));
    }
  }
}


class _StatusCard extends StatelessWidget {
  const _StatusCard({required this.order});

  final FoodOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerHighest.withValues(alpha: 0.55),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _StatusStepper(status: order.status),
            const SizedBox(height: 14),
            Row(
              children: [
                Icon(_statusIcon(order.status), size: 20, color: theme.colorScheme.primary),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    _statusLabel(order.status),
                    style: theme.textTheme.titleSmall,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              _etaText(order.status),
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _StatusStepper extends StatelessWidget {
  const _StatusStepper({required this.status});

  final String status;

  static const List<({String label, IconData icon})> _steps = [
    (label: 'Dibuat', icon: Icons.receipt_long),
    (label: 'Dikonfirmasi', icon: Icons.check),
    (label: 'Dimasak', icon: Icons.soup_kitchen),
    (label: 'Siap', icon: Icons.inventory_2),
    (label: 'Diambil', icon: Icons.inventory),
    (label: 'Diantar', icon: Icons.directions_bike),
    (label: 'Selesai', icon: Icons.check_circle),
  ];

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final idx = kFoodStatusFlow.indexOf(status);
    final done = idx < 0 ? -1 : idx;
    final active = Colors.orange.shade700;
    final inactive = theme.colorScheme.surfaceContainerHighest;
    final outline = theme.colorScheme.outline;

    return Row(
      children: [
        for (var i = 0; i < _steps.length; i++) ...[
          Expanded(
            flex: 2,
            child: _step(step: _steps[i], reached: i < done, isCurrent: i == done,
                active: active, inactive: inactive, outline: outline,
                doneColor: (status == 'DELIVERED' || status == 'SETTLED') && i == done
                    ? Colors.green : active),
          ),
          if (i < _steps.length - 1)
            Expanded(
              flex: 1,
              child: Container(
                height: 2,
                margin: const EdgeInsets.only(bottom: 14),
                color: i < done ? active : inactive,
              ),
            ),
        ],
      ],
    );
  }

  Widget _step({
    required ({String label, IconData icon}) step,
    required bool reached,
    required bool isCurrent,
    required Color active,
    required Color inactive,
    required Color outline,
    required Color doneColor,
  }) {
    final Color bg;
    final Color fg;
    final IconData icon;
    if (reached) {
      bg = doneColor; fg = Colors.white; icon = Icons.check;
    } else if (isCurrent) {
      bg = Colors.white; fg = active; icon = step.icon;
    } else {
      bg = inactive; fg = outline; icon = step.icon;
    }
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        AnimatedContainer(
          duration: const Duration(milliseconds: 300),
          width: 28, height: 28,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: bg,
            border: Border.all(color: isCurrent ? active : Colors.transparent, width: 2.5),
          ),
          child: Icon(icon, color: fg, size: 15),
        ),
        const SizedBox(height: 4),
        Text(step.label, maxLines: 1, overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 8, fontWeight: isCurrent ? FontWeight.bold : FontWeight.normal,
                color: reached ? active : outline)),
      ],
    );
  }
}

class _MerchantCard extends StatelessWidget {
  const _MerchantCard({required this.order});

  final FoodOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: ListTile(
        leading: const CircleAvatar(child: Icon(Icons.storefront)),
        title: const Text('Restoran sedang menyiapkan pesanan Anda'),
        subtitle: Text(_merchantText(order.status)),
        trailing: IconButton(
          tooltip: 'Hubungi restoran',
          onPressed: () => _notImplemented(context, 'Hubungi restoran'),
          icon: const Icon(Icons.call_outlined),
        ),
      ),
    );
  }
}

class _DriverCard extends StatelessWidget {
  const _DriverCard();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: ListTile(
        leading: const CircleAvatar(child: Icon(Icons.directions_bike)),
        title: const Text('Mencari driver terdekat…'),
        subtitle: const Text('Driver akan muncul setelah pesanan siap diambil.'),
        trailing: IconButton(
          tooltip: 'Hubungi driver',
          onPressed: () => _notImplemented(context, 'Hubungi driver'),
          icon: const Icon(Icons.call_outlined),
        ),
      ),
    );
  }
}

class _OrderSummaryCard extends StatelessWidget {
  const _OrderSummaryCard({required this.order});

  final FoodOrder order;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Rincian Pesanan', style: theme.textTheme.titleSmall),
            const SizedBox(height: 8),
            if (order.items.isEmpty)
              Text('(Detail item dimuat saat pembuatan)',
                  style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant)),
            for (final item in order.items)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 2),
                child: Row(
                  children: [
                    Expanded(child: Text('${item.quantity}x ${item.itemName}')),
                    Text(_formatRupiah(item.subtotal)),
                  ],
                ),
              ),
            const Divider(height: 20),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('Total (${order.paymentMethod})',
                    style: theme.textTheme.titleSmall),
                Text(_formatRupiah(order.totalAmount),
                    style: theme.textTheme.titleMedium
                        ?.copyWith(fontWeight: FontWeight.bold)),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _TerminalBox extends StatelessWidget {
  const _TerminalBox({required this.order, required this.onDone});

  final FoodOrder order;
  final VoidCallback onDone;

  @override
  Widget build(BuildContext context) {
    final done = order.status == 'DELIVERED' || order.status == 'SETTLED';
    final color = done ? Colors.green.shade600 : Colors.red.shade600;
    return Column(
      children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          decoration: BoxDecoration(
            color: color.withValues(alpha: 0.1),
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: color),
          ),
          child: Row(
            children: [
              Icon(done ? Icons.check_circle : Icons.cancel, color: color),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  done ? 'Pesanan selesai. Selamat menikmati!' : 'Pesanan dibatalkan.',
                  style: TextStyle(color: color, fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 10),
        OutlinedButton.icon(
          onPressed: onDone,
          icon: const Icon(Icons.home_outlined),
          label: const Text('Kembali ke Beranda'),
        ),
      ],
    );
  }
}

class _CancelButton extends StatelessWidget {
  const _CancelButton({required this.onPressed});

  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final color = Colors.red.shade700;
    return OutlinedButton.icon(
      style: OutlinedButton.styleFrom(
        foregroundColor: color,
        minimumSize: const Size.fromHeight(48),
        side: BorderSide(color: color),
      ),
      onPressed: onPressed,
      icon: const Icon(Icons.cancel_outlined),
      label: const Text('Cancel Order'),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.cloud_off, size: 48, color: Colors.grey),
            const SizedBox(height: 12),
            const Text('Gagal memuat status pesanan'),
            const SizedBox(height: 8),
            Text(message, textAlign: TextAlign.center,
                style: const TextStyle(color: Colors.grey)),
            const SizedBox(height: 16),
            FilledButton.icon(
                onPressed: onRetry, icon: const Icon(Icons.refresh), label: const Text('Coba lagi')),
          ],
        ),
      ),
    );
  }
}

class _MissingArgsView extends StatelessWidget {
  const _MissingArgsView();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Track Order')),
      body: const Center(child: Text('Argumen order tidak ditemukan.')),
    );
  }
}

class _DemoBadge extends StatelessWidget {
  const _DemoBadge();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.only(right: 12),
        child: Tooltip(
          message: 'Mode demo: detail driver & lokasi belum tersedia di backend.',
          child: const Text('DEMO',
              style: TextStyle(color: Colors.orange, fontSize: 12,
                  fontWeight: FontWeight.bold, letterSpacing: 1.2)),
        ),
      ),
    );
  }
}

void _notImplemented(BuildContext context, String feature) {
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(content: Text('$feature: fitur placeholder (TODO).')),
  );
}

const Map<String, String> _statusText = {
  'CREATED': 'Pesanan dibuat',
  'CONFIRMED': 'Restoran mengonfirmasi',
  'PREPARING': 'Sedang disiapkan',
  'READY_FOR_PICKUP': 'Siap diambil',
  'PICKED_UP': 'Driver mengambil',
  'IN_TRANSIT': 'Dalam perjalanan',
  'DELIVERED': 'Pesanan selesai',
  'SETTLED': 'Pembayaran selesai',
  'CANCELLED': 'Pesanan dibatalkan',
};

String _statusLabel(String s) => _statusText[s] ?? s.replaceAll('_', ' ');

IconData _statusIcon(String s) => switch (s) {
  'CREATED' => Icons.receipt_long,
  'CONFIRMED' => Icons.check,
  'PREPARING' => Icons.soup_kitchen,
  'READY_FOR_PICKUP' => Icons.inventory_2,
  'PICKED_UP' => Icons.inventory,
  'IN_TRANSIT' => Icons.directions_bike,
  'DELIVERED' => Icons.check_circle,
  'SETTLED' => Icons.payments,
  'CANCELLED' => Icons.cancel,
  _ => Icons.info_outline,
};

String _merchantText(String s) => switch (s) {
  'PREPARING' => 'Preparing… ~10 menit',
  'READY_FOR_PICKUP' => 'Pesanan siap diambil driver.',
  _ => 'Menunggu pembaruan status.',
};

String _etaText(String s) => switch (s) {
  'CREATED' => 'Menunggu konfirmasi restoran…',
  'CONFIRMED' => 'Restoran telah mengonfirmasi pesanan Anda.',
  'PREPARING' => 'Preparing… sekitar 10-15 menit.',
  'READY_FOR_PICKUP' => 'Menunggu driver mengambil pesanan.',
  'PICKED_UP' => 'Driver telah mengambil pesanan Anda.',
  'IN_TRANSIT' => 'Menuju lokasi pengiriman Anda.',
  'DELIVERED' => 'Pesanan telah sampai.',
  'SETTLED' => 'Pembayaran telah diselesaikan.',
  'CANCELLED' => 'Pesanan telah dibatalkan.',
  _ => '',
};

String _shortId(String id) => id.length > 8 ? id.substring(0, 8).toUpperCase() : id.toUpperCase();

String _formatRupiah(num value) {
  final digits = value.round().toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buffer.write('.');
    buffer.write(digits[i]);
  }
  return 'Rp $buffer';
}
