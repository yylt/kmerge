package resource

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/yylt/kmerge/pkg"
	"github.com/yylt/kmerge/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var minWorkNumber = 3

type res struct {
	t *util.Trigger
	k pkg.Kind

	// namespace/name
	primary string

	// name from annotation
	name string

	// sync from namespace
	// nil mean allnamespace
	fromns *hashset.Set
}

type manager struct {
	client.Client

	ctx context.Context

	// record primary secret ns/name
	data map[string]*res

	ch chan string

	mu sync.RWMutex
}

func NewSecret(mgr ctrl.Manager, ctx context.Context, number int) (*manager, error) {
	n := &manager{
		ctx:    ctx,
		Client: mgr.GetClient(),
		data:   map[string]*res{},
		ch:     make(chan string, 128),
	}
	if number < minWorkNumber {
		number = minWorkNumber
	}
	for i := 0; i < number; i++ {
		go n.processWork()
	}
	err := n.probe(mgr)
	if err != nil {
		return nil, err
	}

	return n, err
}

func (n *manager) probe(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Complete(n)
}

func (n *manager) processWork() {
	for {
		select {
		case ev, ok := <-n.ch:
			if !ok {
				return
			}
			n.mu.RLock()
			se, ok := n.data[ev]
			if !ok {
				n.mu.RUnlock()
				continue
			}
			trigger := se.t
			n.mu.RUnlock()
			trigger.Trigger()

		case <-n.ctx.Done():
			return
		}
	}
}

func (n *manager) pushrsc(name string, nsname types.NamespacedName) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for k, v := range n.data {
		if name != "" && v.name != name {
			continue
		}
		if v.fromns.Size() == 0 || v.fromns.Contains(nsname.Namespace) {
			n.ch <- k
			continue
		}
	}
}

func (n *manager) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		in  = &corev1.Secret{}
		err error
	)

	namespaceName := req.NamespacedName
	if err = n.Get(ctx, namespaceName, in); err != nil {
		klog.Errorf(fmt.Sprintf("faild get secret %s.", namespaceName.Name))
		n.mu.Lock()
		defer n.mu.Unlock()
		delete(n.data, namespaceName.String())
		n.pushrsc("", namespaceName)
		return ctrl.Result{}, nil
	}

	if !in.ObjectMeta.DeletionTimestamp.IsZero() {
		n.mu.Lock()
		defer n.mu.Unlock()
		delete(n.data, namespaceName.String())
		n.pushrsc(in.Annotations[pkg.KmergeNameKey], namespaceName)
		return ctrl.Result{}, nil
	}
	_, hasPrimary := in.Annotations[pkg.KmergePrimaryKey]
	_, hasName := in.Annotations[pkg.KmergeNameKey]

	if !hasName || !hasPrimary {
		n.pushrsc(in.Annotations[pkg.KmergeNameKey], namespaceName)
		return ctrl.Result{}, nil
	}
	nsname := namespaceName.String()
	n.mu.Lock()
	defer n.mu.Unlock()
	klog.Infof("found primary secret %s update", nsname)

	info, ok := n.data[nsname]
	if !ok {
		info = new(res)
		info.fromns = hashset.New()
		info.primary = nsname
		info.name = in.Annotations[pkg.KmergeNameKey]
		info.k = pkg.Textk
		n.data[nsname] = info
	}
	trig, err := util.NewTrigger(util.Parameters{
		Name:        nsname,
		MinInterval: time.Second * 1,
		TriggerFunc: func() {
			n.handle(nsname)
		},
	})
	if err != nil {
		klog.Errorf("prepare trigger %s failed: %v", namespaceName, err)
		return ctrl.Result{}, nil
	}
	info.t = trig

	info.name = in.Annotations[pkg.KmergeNameKey]
	fromns, ok := in.Annotations[pkg.KmergeFromNsKey]
	if ok {
		fns := strings.Split(fromns, ",")
		for _, v := range fns {
			info.fromns.Add(strings.TrimSpace(v))
		}
	}
	kind := in.Annotations[pkg.KmergeTypeKey]
	if kind != "" {
		k, ok := pkg.ValidKind(kind)
		if ok {
			info.k = k
		}
	}
	n.ch <- namespaceName.String()
	return ctrl.Result{}, nil
}

func (n *manager) handle(namespaceName string) {
	name := strings.Split(namespaceName, string(types.Separator))
	if len(name) != 2 {
		return
	}
	klog.Infof("start handle secret %s", namespaceName)

	var (
		nsname = types.NamespacedName{
			Name:      name[1],
			Namespace: name[0],
		}
		in        = &corev1.Secret{}
		mergelist = &corev1.SecretList{}

		infos seInfos
		err   error
	)

	if err = n.Get(n.ctx, nsname, in); err != nil {
		klog.Errorf(fmt.Sprintf("inmegerd, faild get secret(%s): %v", nsname, err))
		return
	}
	se := n.getInfo(namespaceName)
	if se == nil {
		return
	}
	klog.V(2).Infof("secret %s info %+v", namespaceName, se)
	if se.fromns == nil || se.fromns.Size() == 0 {
		err = n.List(n.ctx, mergelist)
		if err != nil {
			klog.Errorf("inmegerd, faild list secret: %v", err)
			return
		}
		infos = append(infos, filter(mergelist, se)...)
	} else {
		for _, v := range se.fromns.Values() {
			ns := v.(string)
			err = n.List(n.ctx, mergelist, &client.ListOptions{Namespace: ns})
			if err != nil {
				klog.Errorf("inmegerd, faild list secret: %v", err)
				return
			}
			infos = append(infos, filter(mergelist, se)...)
			clear(mergelist.Items)
		}
	}
	klog.V(2).Infof("merge list :%v", infos)
	sort.Sort(infos)
	err = n.updateSecret(infos, in, func(s [][]byte) ([]byte, error) {
		switch se.k {
		case pkg.Textk:
			return TextMerge(s)
		case pkg.Jsonk:
			return JsonMerge(s)
		case pkg.Yamlk:
			return YamlMerge(s)
		default:
			return nil, fmt.Errorf("not support")
		}
	})
	klog.Infof("update secret %s, msg: %v", se.primary, err)
}

func (n *manager) getInfo(namespaceName string) *res {
	n.mu.RLock()
	defer n.mu.RUnlock()
	v, ok := n.data[namespaceName]
	if !ok {
		return nil
	}
	return &res{
		name:    v.name,
		primary: v.primary,
		fromns:  hashset.New(v.fromns.Values()...),
		k:       v.k,
	}
}

func filter(ls *corev1.SecretList, rs *res) seInfos {
	if ls == nil || rs == nil {
		return nil
	}
	var (
		ses seInfos
	)
	for _, se := range ls.Items {
		nsname := fmt.Sprintf("%s/%s", se.GetNamespace(), se.GetName())
		if nsname == rs.primary {
			continue
		}
		if se.Annotations[pkg.KmergeNameKey] != rs.name {
			continue
		}
		ses = append(ses, seInfo{
			Secret: se.DeepCopy(),
		})
	}
	return ses
}

func (m *manager) updateSecret(infos seInfos, in *corev1.Secret, fn Mergefn) error {
	var (
		values = map[string]*bytes.Buffer{}

		key = util.NewPrioStringList()

		hash = md5.New()

		vs = [][]byte{}
	)
	if in == nil {
		return nil
	}
	inCopy := in.DeepCopy()
	for k := range inCopy.Data {
		values[k] = util.GetBuf()
		key.Push(k)
	}

	for k, buf := range values {
		clear(vs)
		for _, se := range infos {
			v, ok := se.Data[k]
			if ok {
				vs = append(vs, v)
			}
		}
		v, err := fn(vs)
		if err != nil {
			return err
		}
		buf.Write(v)
	}
	for {
		v, ok := key.Pop()
		if !ok {
			break
		}
		buf, ok := values[v.(string)]
		if !ok {
			continue
		}
		len, err := hash.Write(buf.Bytes())
		if err != nil || len != buf.Len() {
			return fmt.Errorf("copy fail, msg: %v", err)
		}
		inCopy.Data[v.(string)] = buf.Bytes()
		util.PutBuf(buf)
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if inCopy.Annotations[pkg.KmergeHashKey] == sum {
		return nil
	}
	inCopy.Annotations[pkg.KmergeHashKey] = sum
	return util.Backoff(func() error {
		return m.Client.Patch(m.ctx, inCopy, client.MergeFrom(in))
	})
}
