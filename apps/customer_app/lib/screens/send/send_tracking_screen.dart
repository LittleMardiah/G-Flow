import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/send_order.dart';
import '../../models/send_stop.dart';
import '../../providers/send_order_provider.dart';

/// Send Package Tracking (ROADMAP 3.8 G & HALAMAN.txt 5.2) - multi-stop + RTS.
class SendTrackingScreen extends ConsumerWidget {
  const SendTrackingScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final args = _args(context);
    if (args == null) return const _MissingArgsView();

    final state = ref.watch(sendTrackingProvider(args));
    return Scaffold(
      appBar: AppBar(
        title: Text('Send #${_shortId(args.orderId)}'),
        actions: [if (state.isMock) const _DemoBadge()],
      ),
      body: state.order == null
          ? (state.error != null
              ? _ErrorView(
                  message: state.error!,
                  onRetry: () =>
                      ref.read(sendTrackingProvider(args).notifier).retry(),
                )
              : const Center(child: CircularProgressIndicator()))
          : _content(context, ref, args, state),
    );
  }

  Widget _content(BuildContext context, WidgetRef ref,
      SendOrderTrackingArgs args, SendTrackingState state) {
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
        _PackageCard(order: order),
        const SizedBox(height: 14),
        _StatusStepper(status: order.status),
        const SizedBox(height: 14),
        if (order.status == 'PICKED_UP' || order.status == 'IN_TRANSIT')
          const _DriverCard(),
        const SizedBox(height: 14),
        _StopsCard(order: order),
        const SizedBox(height: 20),
        if (order.isTerminal)
          _TerminalBox(order: order, onDone: () => Navigator.of(context).pop())
        else
          _CancelButton(
            onPressed: () => _confirmCancel(context, ref, args),
          ),
      ],
    );
  }

  SendOrderTrackingArgs? _args(BuildContext context) {
    final raw = ModalRoute.of(context)?.settings.arguments;
    return raw is SendOrderTrackingArgs ? raw : null;
  }

  Future<void> _confirmCancel(BuildContext context, WidgetRef ref, args) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (d) => AlertDialog(
        title: const Text('Batalkan pengiriman?'),
        content: const Text(
            'Pembatalan sebelum driver mengambil paket tidak dikenakan biaya. '
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
    final ok = await ref.read(sendTrackingProvider(args).notifier).cancelOrder();
    if (ok && context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Pengiriman dibatalkan.')));
    }
  }
}

class _PackageCard extends StatelessWidget {
  const _PackageCard({required this.order});

  final SendOrder order;

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
            Row(
              children: [
                const Icon(Icons.inventory_2_outlined),
                const SizedBox(width: 8),
                Text('Package: ${order.packageType}',
                    style: theme.textTheme.titleSmall),
              ],
            ),
            const SizedBox(height: 6),
            Text(_statusLabel(order.status),
                style: theme.textTheme.titleMedium
                    ?.copyWith(fontWeight: FontWeight.bold)),
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
    (label: 'Cari Driver', icon: Icons.radar),
    (label: 'Driver', icon: Icons.person_pin_circle),
    (label: 'Diambil', icon: Icons.inventory),
    (label: 'Diantar', icon: Icons.route),
    (label: 'Selesai', icon: Icons.check_circle),
  ];

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final idx = kSendStatusFlow.indexOf(status);
    final done = idx < 0 ? -1 : idx;
    final active = Colors.green.shade700;
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
          width: 30, height: 30,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: bg,
            border: Border.all(color: isCurrent ? active : Colors.transparent, width: 2.5),
          ),
          child: Icon(icon, color: fg, size: 16),
        ),
        const SizedBox(height: 4),
        Text(step.label, maxLines: 1, overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: 9, fontWeight: isCurrent ? FontWeight.bold : FontWeight.normal,
                color: reached ? active : outline)),
      ],
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
        leading: const CircleAvatar(child: Icon(Icons.directions_car)),
        title: const Text('Driver menuju pickup…'),
        subtitle: const Text('Paket akan diambil dari lokasi Anda.'),
        trailing: IconButton(
          tooltip: 'Hubungi driver',
          onPressed: () => _notImplemented(context, 'Hubungi driver'),
          icon: const Icon(Icons.call_outlined),
        ),
      ),
    );
  }
}

class _StopsCard extends StatelessWidget {
  const _StopsCard({required this.order});

  final SendOrder order;

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
            Text('Stop Status', style: theme.textTheme.titleSmall),
            const SizedBox(height: 10),
            if (order.stops.isEmpty)
              Text('(Detail stop dimuat saat pembuatan)',
                  style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant))
            else
              for (final stop in order.stops) _stopTile(context, stop),
          ],
        ),
      ),
    );
  }

  Widget _stopTile(BuildContext context, SendStop stop) {
    final theme = Theme.of(context);
    final done = stop.isDelivered;
    final color = done
        ? Colors.green
        : (stop.isSkipped ? Colors.orange : theme.colorScheme.primary);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(done ? Icons.check_circle : Icons.place,
              color: color, size: 20),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Stop ${stop.order}: ${stop.recipientName}',
                  style: theme.textTheme.bodyMedium,
                ),
                if (stop.locationLabel case final location?)
                  Text(location,
                      style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant)),
                if (stop.distanceKm > 0)
                  Text('${stop.distanceKm.toStringAsFixed(1)} km · ${_formatRupiah(stop.allocatedFare)}',
                      style: theme.textTheme.bodySmall),
              ],
            ),
          ),
          Chip(
            label: Text(stop.status.replaceAll('_', ' '),
                style: const TextStyle(fontSize: 10)),
            visualDensity: VisualDensity.compact,
            backgroundColor: color.withValues(alpha: 0.12),
            labelStyle: TextStyle(color: color),
          ),
        ],
      ),
    );
  }
}

class _TerminalBox extends StatelessWidget {
  const _TerminalBox({required this.order, required this.onDone});

  final SendOrder order;
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
                  done ? 'Paket telah diantar. Terima kasih!' : 'Pengiriman dibatalkan.',
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
      label: const Text('Cancel Send'),
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
            const Text('Gagal memuat status pengiriman'),
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
      appBar: AppBar(title: const Text('Send Tracking')),
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
  ScaffoldMessenger.of(context)
      .showSnackBar(SnackBar(content: Text('$feature: fitur placeholder (TODO).')));
}

const Map<String, String> _statusText = {
  'CREATED': 'Pesanan dibuat',
  'SEARCHING_DRIVER': 'Mencari driver',
  'DRIVER_ASSIGNED': 'Driver ditugaskan',
  'PICKED_UP': 'Paket diambil',
  'IN_TRANSIT': 'Dalam perjalanan',
  'DELIVERED': 'Paket selesai diantar',
  'SETTLED': 'Pembayaran selesai',
  'RETURN_REQUIRED': 'Return required',
  'RETURNED_TO_SENDER': 'Dikembalikan ke pengirim',
  'CANCELLED': 'Dibatalkan',
};

String _statusLabel(String s) => _statusText[s] ?? s.replaceAll('_', ' ');

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
